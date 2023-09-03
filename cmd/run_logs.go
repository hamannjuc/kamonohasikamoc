package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/dory-engine/dory-ctl/pkg"
	"github.com/spf13/cobra"
	"net/http"
	"strings"
)

type OptionsRunLog struct {
	*OptionsCommon `yaml:"optionsCommon" json:"optionsCommon" bson:"optionsCommon" validate:""`
	Param          struct {
		RunName string `yaml:"runName" json:"runName" bson:"runName" validate:""`
	}
}

func NewOptionsRunLog() *OptionsRunLog {
	var o OptionsRunLog
	o.OptionsCommon = OptCommon
	return &o
}

func NewCmdRunLog() *cobra.Command {
	o := NewOptionsRunLog()

	msgUse := fmt.Sprintf("logs [runName]")
	msgShort := fmt.Sprintf("get pipeline run logs")
	msgLong := fmt.Sprintf(`get pipeline run logs in dory-core server`)
	msgExample := fmt.Sprintf(`  # get pipeline run logs
  doryctl run logs test-project1-develop-1`)

	cmd := &cobra.Command{
		Use:                   msgUse,
		DisableFlagsInUseLine: true,
		Short:                 msgShort,
		Long:                  msgLong,
		Example:               msgExample,
		Run: func(cmd *cobra.Command, args []string) {
			CheckError(o.Validate(args))
			CheckError(o.Run(args))
		},
	}

	CheckError(o.Complete(cmd))
	return cmd
}

func (o *OptionsRunLog) Complete(cmd *cobra.Command) error {
	var err error

	err = o.GetOptionsCommon()
	if err != nil {
		return err
	}

	return err
}

func (o *OptionsRunLog) Validate(args []string) error {
	var err error

	err = o.GetOptionsCommon()
	if err != nil {
		return err
	}

	if len(args) != 1 {
		err = fmt.Errorf("runName error: only accept one runName")
		return err
	}

	s := args[0]
	s = strings.Trim(s, " ")
	err = pkg.ValidateMinusNameID(s)
	if err != nil {
		err = fmt.Errorf("runName error: %s", err.Error())
		return err
	}
	o.Param.RunName = s
	return err
}

func (o *OptionsRunLog) Run(args []string) error {
	var err error

	bs, _ := pkg.YamlIndent(o)
	log.Debug(fmt.Sprintf("command options:\n%s", string(bs)))

	param := map[string]interface{}{}
	result, _, err := o.QueryAPI(fmt.Sprintf("api/cicd/run/%s", o.Param.RunName), http.MethodGet, "", param, false)
	if err != nil {
		return err
	}
	run := pkg.Run{}
	err = json.Unmarshal([]byte(result.Get("data.run").Raw), &run)
	if err != nil {
		return err
	}

	if run.RunName == "" {
		err = fmt.Errorf("runName %s not exists", o.Param.RunName)
		return err
	}

	url := fmt.Sprintf("api/ws/log/run/%s", o.Param.RunName)
	err = o.QueryWebsocket(url, o.Param.RunName, []string{})
	if err != nil {
		return err
	}

	return err
}
