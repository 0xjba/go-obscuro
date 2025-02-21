package vkhandler

import (
	"crypto/rand"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/obscuronet/go-obscuro/go/common/viewingkey"

	gethcommon "github.com/ethereum/go-ethereum/common"
)

var ErrInvalidAddressSignature = fmt.Errorf("invalid viewing key signature for requested address")

// Used when the result to an eth_call is equal to nil. Attempting to encrypt then decrypt nil using ECIES throws an exception.
var placeholderResult = []byte("0x")

// VKHandler handles encryption and validation of viewing keys
type VKHandler struct {
	publicViewingKey *ecies.PublicKey
}

// New creates a new viewing key handler
// checks if the signature is valid
// as well if signature matches account address
// todo (@ziga) - function now accepts both old and new messages
func New(requestedAddr *gethcommon.Address, vkPubKeyBytes, accountSignatureHexBytes []byte) (*VKHandler, error) {
	// Recalculate the message signed by MetaMask.
	msgToSign := viewingkey.GenerateSignMessageOG(vkPubKeyBytes, requestedAddr)

	// We recover the key based on the signed message and the signature.
	recoveredAccountPublicKey, err := crypto.SigToPub(accounts.TextHash([]byte(msgToSign)), accountSignatureHexBytes)
	if err != nil {
		return nil, fmt.Errorf("viewing key but could not validate its signature - %w", err)
	}
	recoveredAccountAddress := crypto.PubkeyToAddress(*recoveredAccountPublicKey)

	// We recover the key based on the signed message and the signature (same as before, but with legacy message format "vk"+<vk>"
	// todo (@ziga) remove this once old WE message format is deprecated
	msgToSignLegacy := viewingkey.GenerateSignMessage(vkPubKeyBytes)
	recoveredAccountPublicKeyLegacy, err := crypto.SigToPub(accounts.TextHash([]byte(msgToSignLegacy)), accountSignatureHexBytes)
	if err != nil {
		return nil, fmt.Errorf("viewing key but could not validate its signature - %w", err)
	}
	recoveredAccountAddressLegacy := crypto.PubkeyToAddress(*recoveredAccountPublicKeyLegacy)

	// is the requested account address the same as the address recovered from the signature
	// todo (@ziga) - we currently check also for legacy address and allow both (remove this after transition period)
	if requestedAddr.Hash() != recoveredAccountAddress.Hash() &&
		requestedAddr.Hash() != recoveredAccountAddressLegacy.Hash() {
		return nil, ErrInvalidAddressSignature
	}

	// We decompress the viewing key and create the corresponding ECIES key.
	viewingKey, err := crypto.DecompressPubkey(vkPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decompress viewing key bytes - %w", err)
	}

	return &VKHandler{
		publicViewingKey: ecies.ImportECDSAPublic(viewingKey),
	}, nil
}

// Encrypt returns the payload encrypted with the viewingKey
func (m *VKHandler) Encrypt(bytes []byte) ([]byte, error) {
	if len(bytes) == 0 {
		bytes = placeholderResult
	}

	encryptedBytes, err := ecies.Encrypt(rand.Reader, m.publicViewingKey, bytes, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt with given public VK - %w", err)
	}

	return encryptedBytes, nil
}
