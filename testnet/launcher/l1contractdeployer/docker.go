package l1contractdeployer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/obscuronet/go-obscuro/go/node"
	"github.com/sanity-io/litter"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/obscuronet/go-obscuro/go/common/docker"
)

type ContractDeployer struct {
	cfg         *Config
	containerID string
}

func NewDockerContractDeployer(cfg *Config) (*ContractDeployer, error) {
	return &ContractDeployer{
		cfg: cfg,
	}, nil // todo (@pedro) - add validation
}

func (n *ContractDeployer) Start() error {
	fmt.Printf("Starting L1 contract deployer with config: \n%s\n\n", litter.Sdump(*n.cfg))

	cmds := []string{
		"npx", "hardhat", "deploy",
		"--network", "layer1",
	}

	envs := map[string]string{
		"NETWORK_JSON": fmt.Sprintf(`
{ 
        "layer1" : {
            "url" : "%s",
            "live" : false,
            "saveDeployments" : true,
            "deploy": [ 
                "deployment_scripts/core"
            ],
            "accounts": [ "%s" ]
        }
    }
`, n.cfg.l1HTTPURL, n.cfg.privateKey),
	}

	containerID, err := docker.StartNewContainer("hh-l1-deployer", n.cfg.dockerImage, cmds, nil, envs, nil, nil)
	if err != nil {
		return err
	}
	n.containerID = containerID
	return nil
}

func (n *ContractDeployer) RetrieveL1ContractAddresses() (*node.NetworkConfig, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	// make sure the container has finished execution
	err = docker.WaitForContainerToFinish(n.containerID, time.Minute)
	if err != nil {
		return nil, err
	}

	logsOptions := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "3",
	}

	// Read the container logs
	out, err := cli.ContainerLogs(context.Background(), n.containerID, logsOptions)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	// Buffer the output
	var buf bytes.Buffer
	_, err = io.Copy(&buf, out)
	if err != nil {
		return nil, err
	}

	// Get the last three lines
	output := buf.String()
	lines := strings.Split(output, "\n")

	managementAddr, err := findAddress(lines[0])
	if err != nil {
		return nil, err
	}
	messageBusAddr, err := findAddress(lines[1])
	if err != nil {
		return nil, err
	}
	l1BlockHash := readValue("L1Start", lines[2])

	return &node.NetworkConfig{
		ManagementContractAddress: managementAddr,
		MessageBusAddress:         messageBusAddr,
		L1StartHash:               l1BlockHash,
	}, nil
}

func findAddress(line string) (string, error) {
	// Regular expression to match Ethereum addresses
	re := regexp.MustCompile("(0x[a-fA-F0-9]{40})")

	// Find all Ethereum addresses in the text
	matches := re.FindAllString(line, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no address found in: %s", line)
	}
	// Print the last
	return matches[len(matches)-1], nil
}

func readValue(name string, line string) string {
	parts := strings.Split(line, fmt.Sprintf("%s=", name))
	val := strings.TrimSpace(parts[len(parts)-1])
	return val
}
