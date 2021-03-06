package pkg

import "embed"

const (
	VersionDoryCtl       = "v0.8.2"
	VersionDoryCore      = "v1.7.1"
	VersionDoryDashboard = "v1.6.5"
	BaseCmdName          = "doryctl"
	ConfigDirDefault     = ".doryctl"
	ConfigFileDefault    = "config.yaml"
	EnvVarConfigFile     = "DORYCONFIG"
	DirInstallScripts    = "install_scripts"
	DirInstallConfigs    = "install_configs"

	TimeoutDefault = 5

	LogTypeInfo    = "INFO"
	LogTypeWarning = "WARNING"
	LogTypeError   = "ERROR"

	StatusSuccess = "SUCCESS"
	StatusFail    = "FAIL"

	InputValueAbort   = "ABORT"
	InputValueConfirm = "CONFIRM"

	LogStatusInput = "INPUT" // special usage for websocket send notice directives
)

var (
	// !!! go embed function will ignore _* and .* file
	//go:embed install_scripts/* install_scripts/kubernetes/harbor/.helmignore install_scripts/kubernetes/harbor/templates/_helpers.tpl
	FsInstallScripts embed.FS
	//go:embed install_configs/*
	FsInstallConfigs embed.FS

	DefCmdKinds = map[string]string{
		"all":      "",
		"build":    "buildDefs",
		"package":  "packageDefs",
		"deploy":   "deployContainerDefs",
		"step":     "customStepDef",
		"pipeline": "pipelineDef",
		"ops":      "customOpsDefs",
		"ignore":   "dockerIgnoreDefs",
	}

	AdminCmdKinds = map[string]string{
		"all":    "",
		"user":   "user",
		"step":   "customStepConf",
		"env":    "envK8s",
		"comtpl": "componentTemplate",
	}
)
