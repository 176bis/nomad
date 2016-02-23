package driver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/client/driver/logcollector"
)

// createExecutor launches an executor plugin and returns an instance of the
// Executor interface
func createExecutor(config *plugin.ClientConfig, w io.Writer,
	clientConfig *config.Config) (executor.Executor, *plugin.Client, error) {
	config.HandshakeConfig = HandshakeConfig
	config.Plugins = GetPluginMap(w)
	config.MaxPort = clientConfig.ClientMaxPort
	config.MinPort = clientConfig.ClientMinPort

	// setting the setsid of the plugin process so that it doesn't get signals sent to
	// the nomad client.
	if config.Cmd != nil {
		isolateCommand(config.Cmd)
	}

	executorClient := plugin.NewClient(config)
	rpcClient, err := executorClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for executor plugin: %v", err)
	}

	raw, err := rpcClient.Dispense("executor")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the executor plugin: %v", err)
	}
	executorPlugin := raw.(executor.Executor)
	return executorPlugin, executorClient, nil
}

func createLogCollector(config *plugin.ClientConfig, w io.Writer,
	clientConfig *config.Config) (logcollector.LogCollector, *plugin.Client, error) {
	config.HandshakeConfig = HandshakeConfig
	config.Plugins = GetPluginMap(w)
	config.MaxPort = clientConfig.ClientMaxPort
	config.MinPort = clientConfig.ClientMinPort
	if config.Cmd != nil {
		isolateCommand(config.Cmd)
	}

	syslogClient := plugin.NewClient(config)
	rpcCLient, err := syslogClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for syslog plugin: %v", err)
	}

	raw, err := rpcCLient.Dispense("syslogcollector")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the syslog plugin: %v", err)
	}
	logCollector := raw.(logcollector.LogCollector)
	return logCollector, syslogClient, nil
}

// killProcess kills a process with the given pid
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// destroyPlugin kills the plugin with the given pid and also kills the user
// process
func destroyPlugin(pluginPid int, userPid int) error {
	var merr error
	if err := killProcess(pluginPid); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := killProcess(userPid); err != nil {
		merr = multierror.Append(merr, err)
	}
	return merr
}

// validateCommand validates that the command only has a single value and
// returns a user friendly error message telling them to use the passed
// argField.
func validateCommand(command, argField string) error {
	trimmed := strings.TrimSpace(command)
	if len(trimmed) == 0 {
		return fmt.Errorf("command empty: %q", command)
	}

	if len(trimmed) != len(command) {
		return fmt.Errorf("command contains extra white space: %q", command)
	}

	split := strings.Split(trimmed, " ")
	if len(split) != 1 {
		return fmt.Errorf("command contained more than one input. Use %q field to pass arguments", argField)
	}

	return nil
}
