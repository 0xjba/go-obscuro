# Wallet extension design

## Scope

The design for the wallet extension, a component that is responsible for handling RPC requests from traditional 
Ethereum wallets (e.g. MetaMask, hardware wallets), tooling (e.g. Remix) and webapps to the Obscuro host.

## Requirements

* The wallet extension serves the following APIs:
  * The APIs defined in the [Ethereum JSON-RPC specification
    ](https://playground.open-rpc.org/?schemaUrl=https://raw.githubusercontent.com/ethereum/eth1.0-apis/assembled-spec/openrpc.json)
  * Additional APIs as needed to support common tools (e.g. MetaMask's use of Geth's `net_version` API)
* The wallet extension is run locally by the end user
* Encryption
  * The wallet extension does not send or receive any sensitive information in plaintext. The following methods are 
    considered sensitive (this list may grow over time):
    * `eth_call`
    * `eth_getBalance`
    * `eth_getTransactionByHash`
    * `eth_getTransactionReceipt`
    * `eth_sendRawTransaction`
  * The keys used to encrypt sensitive information (the *viewing keys*) are stored solely on the client-side (e.g. 
    wallet, webapp), and are not shared with any third-parties (e.g. the node operator)
* Revocation
  * Viewing keys can be permanently revoked
* UX
  * Any encryption is transparent to the client; from the client's perspective, they are interacting with a "standard" 
      non-encrypting implementation of the Ethereum JSON-RPC specification
  * The wallet extension is usable by any webapp, tool or wallet type, and in particular:
    * Hardware wallets that do not offer a decryption capability
    * MetaMask, for which the keys are only available when running in the browser
  * Restarting the wallet extension does not require existing viewing keys to be regenerated
  * Existing viewing keys are automatically resubmitted if the network is restarted
  * Multiple wallet extensions can run in parallel, with separate sets of viewing keys and optionally connected to 
    separate hosts 
* Some tooling (e.g. MetaMask) does not set the `from` field in `eth_call` requests. For any received `eth_call` 
  request, it is acceptable for the wallet extension to set the `from` field programmatically (e.g. to enable 
  encryption of the response)

## Non-requirements

* Encryption or obfuscation of viewing keys at rest. It is acceptable to store viewing keys locally in the clear

## Design

The wallet extension is a local server application that maintains an RPC connection to one or more Obscuro hosts. It 
serves two endpoints:

* An endpoint for managing viewing keys
* An endpoint that meets the Ethereum JSON-RPC specification

### Viewing keys

Viewing keys are keypairs that are generated by the wallet extension to allow the encryption and decryption of 
sensitive requests.

Each viewing key is signed by an account key (e.g. one stored in MetaMask), to prove which account it is associated 
with. This means that sensitive information can safely be encrypted with the viewing key, because we know the viewing 
key must have been "authorised" by the key that owns the account.

### Viewing-keys endpoint

This endpoint serves a webpage where the end user can generate new viewing keys for their account. For each generation 
event, the following steps are taken:

* The wallet extension generates a new keypair
* The wallet extension stores the private key locally, tagged with the address it is associated with
* The end user signs a payload containing the public key and some metadata using an account key in their wallet (e.g. 
  MetaMask), proving that the viewing key is "authorised" by the account in question
* The wallet extension sends the public key and signature to the Obscuro enclave via the Obscuro host over RPC
* The Obscuro enclave checks the signature
* The Obscuro enclave stores the public key locally, tagged with the address it is associated with

Whenever an enclave needs to send sensitive information to the end user (e.g. a transaction result or account balance), 
it encrypts the sensitive information with the viewing key associated with the account. This ensures that the sensitive 
information can only be decrypted by the wallet extension.

By generating new viewing keys through a webpage, we maintain compatibility with MetaMask, since we can use the 
MetaMask extension to ask people to sign the new viewing public key in-browser.

Viewing keys are persisted to a file in the user's home directory in cleartext. The viewing keys are stored as 
`(<host_url_and_port>, <account>, <viewing_key>, <signature>)` quadruples. When the wallet extension is restarted, any 
viewing keys for the configured host (as identified by the URL and port) are loaded and resubmitted to the enclave.

### Ethereum JSON-RPC endpoint

The wallet extension serves a standard implementation of the Ethereum JSON-RPC specification, except in the following 
respects:

* The wallet extension encrypts any request containing sensitive information with the Obscuro enclave public key before 
  forwarding it to the Obscuro host
* The enclave encrypts any response containing sensitive information with the viewing public key for the address 
  *associated* with that request (see below)
* The wallet extension decrypts any encrypted responses with the viewing private key before forwarding them on to the 
  user

This ensures that the encryption and decryption involved in the Obscuro protocol is transparent to the end user, and 
that we are not relying on decryption capabilities being available in the wallet.

How do we determine which address is associated with a given request?

* `eth_call`: We use the address in the `from` field
* `eth_getBalance`: We use the address for which the balance is requested
* `eth_getTransactionByHash`, `eth_getTransactionReceipt` and `eth_sendRawTransaction`: We use the address of the 
  signer of the transaction

### Handling `eth_call` requests

Certain tools (e.g. MetaMask) do not set the `from` field in `eth_call` requests, since this field is marked as 
optional in the Ethereum standard. However, this is problematic for Obscuro, since we need to use this field to 
determine which viewing public key to use to encrypt the response.

We therefore attempt to set any missing `from` fields in `eth_call` requests programmatically, as follows:

* If the `from` field is set, it is left untouched
* Else, if there is a single viewing key registered, we set the `from` field to that viewing key's address
* Else, we search the `data` field for possible addresses. If one of the addresses matching one of the registered
  viewing key's addresses, we set the `from` field to that
* Else, the `from` field is left untouched

## Known limitations

* An additional set of keys, the viewing keys, must be managed outside of the user's wallet
  * Note, however, that these keys are far less sensitive than the signing keys; they only allow an attacker to view, 
    but not modify, the user's ledger
* The end user must run an additional component; this precludes mobile-only wallets
* Having the viewing keys stored by the enclave introduces a denial-of-service vector; the enclave could be induced to 
  store enormous amounts of keys by the attacker. We may have to investigate mitigations (e.g. expiry of keys held by 
  the enclave, with resubmission of expired keys from the wallet extension as needed)
* In the current design, each enclave can only store one viewing key per address. This can be burdensome in some 
  scenarios (e.g. using the same account on two separate machines at the same time)

## Alternatives considered

### Alternatives to a local server application

#### Chrome and Firefox extension

This is appealing because the user experience of installing and running a browser extension is better than the user 
experience of installing and running a local application.

However, this approach is unworkable because Chrome extensions can only make outbound network requests, and cannot 
serve an endpoint. Chrome has a [`chrome.socket` API](https://developer.chrome.com/docs/extensions/reference/socket/), 
but it is only available to Chrome apps, and not extensions.

The penetration of Firefox was considered to be too low for a Firefox-extension-only approach to be considered viable.

#### Chrome app

Chrome apps are deprecated, and their support status is uncertain (see 
[here](https://blog.chromium.org/2021/10/extending-chrome-app-support-on-chrome.html)).

#### MetaMask snap

Snaps are an experimental MetaMask capability.

Snaps have several downsides:

* Snaps need to be installed per-page, requiring a code change in every webapp to prompt the user to install the Obscuro 
  snap
* Snaps are only compatible with MetaMask
* Snaps are marked as experimental and require users to switch from MetaMask to the experimental MetaMask Flask

### Alternatives to using viewing keys

Instead of viewing keys, we could encrypt and decrypt using the end user's existing account key.

This would obviate the need for us to implement a key management solution.

However, it has several downsides:

* It does not support hardware wallets, since hardware wallets do not provide an API to decrypt using their account 
  private key
* It may not be possible with MetaMask - the local server application would somehow have to speak to the browser to 
  access the MetaMask API; this may be possible through a combination of a Chrome extension and websockets, but is 
  unproven
* It would lead to a high number of MetaMask prompts
