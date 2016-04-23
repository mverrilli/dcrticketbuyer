// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	flags "github.com/btcsuite/go-flags"
	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrutil"
)

const (
	defaultConfigFilename = "ticketbuyer.conf"
	defaultLogLevel       = "info"
	defaultLogDirname     = "logs"
	defaultLogFilename    = "ticketbuyer.log"
	currentVersion        = 1
)

var curDir, _ = os.Getwd()
var activeNet = &chaincfg.MainNetParams

var (
	dcrdHomeDir              = dcrutil.AppDataDir("dcrd", false)
	dcrwalletHomeDir         = dcrutil.AppDataDir("dcrwallet", false)
	defaultDaemonRPCKeyFile  = filepath.Join(dcrdHomeDir, "rpc.key")
	defaultDaemonRPCCertFile = filepath.Join(dcrdHomeDir, "rpc.cert")
	defaultConfigFile        = filepath.Join(curDir, defaultConfigFilename)
	defaultWalletRPCKeyFile  = filepath.Join(dcrwalletHomeDir, "rpc.key")
	defaultWalletRPCCertFile = filepath.Join(dcrwalletHomeDir, "rpc.cert")
	defaultLogDir            = filepath.Join(curDir, defaultLogDirname)

	defaultAccountName       = "default"
	defaultTicketAddress     = ""
	defaultPoolAddress       = ""
	defaultMaxFee            = 1.0
	defaultMinFee            = 0.01
	defaultMaxPrice          = 100.0
	defaultMaxPerBlock       = 3
	defaultBalanceToMaintain = 0.0
	defaultHighPricePenalty  = 1.3
	defaultBlocksToAvg       = 11
	defaultFeeTargetScaling  = 1.05
	defaultWaitForTickets    = true
	defaultExpiryDelta       = 16
)

type config struct {
	// General application behavior
	ConfigFile  string `short:"C" long:"configfile" description:"Path to configuration file"`
	ShowVersion bool   `short:"V" long:"version" description:"Display version information and exit"`
	TestNet     bool   `long:"testnet" description:"Use the test network (default mainnet)"`
	SimNet      bool   `long:"simnet" description:"Use the simulation test network (default mainnet)"`
	DebugLevel  string `short:"d" long:"debuglevel" description:"Logging level {trace, debug, info, warn, error, critical}"`
	LogDir      string `long:"logdir" description:"Directory to log output"`

	// RPC client options
	DcrdUser         string `long:"dcrduser" description:"Daemon RPC user name"`
	DcrdPass         string `long:"dcrdpass" description:"Daemon RPC password"`
	DcrdServ         string `long:"dcrdserv" description:"Hostname/IP and port of dcrd RPC server to connect to (default localhost:9109, testnet: localhost:19109, simnet: localhost:18556)"`
	DcrdCert         string `long:"dcrdcert" description:"File containing the dcrd certificate file"`
	DcrwUser         string `long:"dcrwuser" description:"Wallet RPC user name"`
	DcrwPass         string `long:"dcrwpass" description:"Wallet RPC password"`
	DcrwServ         string `long:"dcrwserv" description:"Hostname/IP and port of dcrwallet RPC server to connect to (default localhost:9110, testnet: localhost:19110, simnet: localhost:18557)"`
	DcrwCert         string `long:"dcrwcert" description:"File containing the dcrwallet certificate file"`
	DisableClientTLS bool   `long:"noclienttls" description:"Disable TLS for the RPC client -- NOTE: This is only allowed if the RPC client is connecting to localhost"`

	// Automatic ticket buyer settings
	AccountName       string  `long:"accountname" description:"Name of the account to buy tickets from (default: default)"`
	TicketAddress     string  `long:"ticketaddress" description:"Address to give ticket voting rights to"`
	PoolAddress       string  `long:"pooladdress" description:"Address to give pool fees rights to"`
	PoolFees          float64 `long:"poolfees" description:"The pool fee base rate for a given pool"`
	MaxPrice          float64 `long:"maxprice" description:"Maximum price to pay for a ticket (default: 100)"`
	MaxFee            float64 `long:"maxfee" description:"Maximum ticket fee per KB (default: 1.0)"`
	MinFee            float64 `long:"minfee" description:"Minimum ticket fee per KB (default: 0.01)"`
	MaxPerBlock       int     `long:"maxperblock" description:"Maximum tickets per block (default: 3)"`
	BalanceToMaintain float64 `long:"balancetomaintain" description:"Balance to try to maintain in the wallet"`
	HighPricePenalty  float64 `long:"highpricepenality" description:"The exponential penalty to apply to the number of tickets to purchase above the mean ticket pool price (default: 1.3)"`
	BlocksToAvg       int     `long:"blockstoavg" description:"Number of blocks to average for fees calculation (default: 11)"`
	FeeTargetScaling  float64 `long:"feetargetscaling" description:"The amount above the mean fee in the previous blocks to purchase tickets with, proportional (default: 1.05)"`
	WaitForTickets    bool    `long:"waitfortickets" description:"Wait until your last round of tickets have entered the blockchain to attempt to purchase more (default: true)"`
	ExpiryDelta       int     `long:"expirydelta" description:"Number of blocks in the future before the ticket expires (default: 16)"`
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(dcrwalletHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows cmd.exe-style
	// %VARIABLE%, but they variables can still be expanded via POSIX-style
	// $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// validLogLevel returns whether or not logLevel is a valid debug log level.
func validLogLevel(logLevel string) bool {
	switch logLevel {
	case "trace":
		fallthrough
	case "debug":
		fallthrough
	case "info":
		fallthrough
	case "warn":
		fallthrough
	case "error":
		fallthrough
	case "critical":
		return true
	}
	return false
}

// supportedSubsystems returns a sorted slice of the supported subsystems for
// logging purposes.
func supportedSubsystems() []string {
	// Convert the subsystemLoggers map keys to a slice.
	subsystems := make([]string, 0, len(subsystemLoggers))
	for subsysID := range subsystemLoggers {
		subsystems = append(subsystems, subsysID)
	}

	// Sort the subsytems for stable display.
	sort.Strings(subsystems)
	return subsystems
}

// parseAndSetDebugLevels attempts to parse the specified debug level and set
// the levels accordingly.  An appropriate error is returned if anything is
// invalid.
func parseAndSetDebugLevels(debugLevel string) error {
	// When the specified string doesn't have any delimters, treat it as
	// the log level for all subsystems.
	if !strings.Contains(debugLevel, ",") && !strings.Contains(debugLevel, "=") {
		// Validate debug log level.
		if !validLogLevel(debugLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, debugLevel)
		}

		// Change the logging level for all subsystems.
		setLogLevels(debugLevel)

		return nil
	}

	// Split the specified string into subsystem/level pairs while detecting
	// issues and update the log levels accordingly.
	for _, logLevelPair := range strings.Split(debugLevel, ",") {
		if !strings.Contains(logLevelPair, "=") {
			str := "The specified debug level contains an invalid " +
				"subsystem/level pair [%v]"
			return fmt.Errorf(str, logLevelPair)
		}

		// Extract the specified subsystem and log level.
		fields := strings.Split(logLevelPair, "=")
		subsysID, logLevel := fields[0], fields[1]

		// Validate subsystem.
		if _, exists := subsystemLoggers[subsysID]; !exists {
			str := "The specified subsystem [%v] is invalid -- " +
				"supported subsytems %v"
			return fmt.Errorf(str, subsysID, supportedSubsystems())
		}

		// Validate log level.
		if !validLogLevel(logLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, logLevel)
		}

		setLogLevel(subsysID, logLevel)
	}

	return nil
}

// loadConfig initializes and parses the config using a config file and command
// line options.
func loadConfig() (*config, error) {
	loadConfigError := func(err error) (*config, error) {
		return nil, err
	}

	// Default config.
	cfg := config{
		DebugLevel:        defaultLogLevel,
		ConfigFile:        defaultConfigFile,
		LogDir:            defaultLogDir,
		DcrdCert:          defaultDaemonRPCCertFile,
		DcrwCert:          defaultWalletRPCCertFile,
		AccountName:       defaultAccountName,
		TicketAddress:     defaultTicketAddress,
		PoolAddress:       defaultPoolAddress,
		MaxFee:            defaultMaxFee,
		MinFee:            defaultMinFee,
		MaxPrice:          defaultMaxPrice,
		MaxPerBlock:       defaultMaxPerBlock,
		BalanceToMaintain: defaultBalanceToMaintain,
		HighPricePenalty:  defaultHighPricePenalty,
		BlocksToAvg:       defaultBlocksToAvg,
		FeeTargetScaling:  defaultFeeTargetScaling,
		WaitForTickets:    defaultWaitForTickets,
		ExpiryDelta:       defaultExpiryDelta,
	}

	// A config file in the current directory takes precedence.
	exists := false
	if _, err := os.Stat(defaultConfigFilename); !os.IsNotExist(err) {
		exists = true
	}

	if exists {
		cfg.ConfigFile = defaultConfigFile
	}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.Default)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			preParser.WriteHelp(os.Stderr)
		}
		return loadConfigError(err)
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", currentVersion)
		os.Exit(0)
	}

	// Load additional config from file.
	var configFileError error
	parser := flags.NewParser(&cfg, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintln(os.Stderr, err)
			parser.WriteHelp(os.Stderr)
			return loadConfigError(err)
		}
		configFileError = err
	}

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return loadConfigError(err)
	}

	// The user must set the dcrd and dcrwallet server variables.
	if cfg.DcrdServ == "" {
		return loadConfigError(fmt.Errorf("No dcrd RPC server specified"))
	}
	if cfg.DcrwServ == "" {
		return loadConfigError(fmt.Errorf("No dcrwallet RPC server specified"))
	}

	// Warn about missing config file after the final command line parse
	// succeeds.  This prevents the warning on help messages and invalid
	// options.
	if configFileError != nil {
		log.Warnf("%v", configFileError)
	}

	// Choose the active network params based on the selected network.
	// Multiple networks can't be selected simultaneously.
	numNets := 0
	activeNet = &chaincfg.MainNetParams
	if cfg.TestNet {
		activeNet = &chaincfg.TestNetParams
		numNets++
	}
	if cfg.SimNet {
		activeNet = &chaincfg.SimNetParams
		numNets++
	}
	if numNets > 1 {
		str := "%s: The testnet and simnet params can't be used " +
			"together -- choose one"
		err := fmt.Errorf(str, "loadConfig")
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return loadConfigError(err)
	}

	// Append the network type to the log directory so it is "namespaced"
	// per network.
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, activeNet.Name)

	// Special show command to list supported subsystems and exit.
	if cfg.DebugLevel == "show" {
		fmt.Println("Supported subsystems", supportedSubsystems())
		os.Exit(0)
	}

	// Initialize logging at the default logging level.
	initSeelogLogger(filepath.Join(cfg.LogDir, defaultLogFilename))
	setLogLevels(defaultLogLevel)

	// Parse, validate, and set debug log level(s).
	if err := parseAndSetDebugLevels(cfg.DebugLevel); err != nil {
		err := fmt.Errorf("%s: %v", "loadConfig", err.Error())
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return loadConfigError(err)
	}

	return &cfg, nil
}
