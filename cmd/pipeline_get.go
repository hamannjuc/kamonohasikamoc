package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/dory-engine/dory-ctl/pkg"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"strings"
)

type OptionsPipelineGet struct {
	*OptionsCommon `yaml:"optionsCommon" json:"optionsCommon" bson:"optionsCommon" validate:""`
	ProjectNames   string `yaml:"projectNames" json:"projectNames" bson:"projectNames" validate:""`
	Output         string `yaml:"output" json:"output" bson:"output" validate:""`
	Param          struct {
		ProjectNames  []string `yaml:"projectNames" json:"projectNames" bson:"projectNames" validate:""`
		PipelineNames []string `yaml:"pipelineNames" json:"pipelineNames" bson:"pipelineNames" validate:""`
	}
}

func NewOptionsPipelineGet() *OptionsPipelineGet {
	var o OptionsPipelineGet
	o.OptionsCommon = OptCommon
	return &o
}

func NewCmdPipelineGet() *cobra.Command {
	o := NewOptionsPipelineGet()

	msgUse := fmt.Sprintf("get [pipelineName] ...")
	msgShort := fmt.Sprintf("get pipeline resoures")
	msgLong := fmt.Sprintf(`get pipeline resources in dory-core server`)
	msgExample := fmt.Sprintf(`  # get all pipeline resoures
  doryctl pipeline get
  # get single pipeline resoure
  doryctl pipeline get test-project1-develop
  # get multiple pipeline resoures
  doryctl pipeline get test-project1-develop test-project1-ops`)

	cmd := &cobra.Command{
		Use:                   msgUse,
		DisableFlagsInUseLine: true,
		Short:                 msgShort,
		Long:                  msgLong,
		Example:               msgExample,
		Run: func(cmd *cobra.Command, args []string) {
			CheckError(o.Complete(cmd))
			CheckError(o.Validate(args))
			CheckError(o.Run(args))
		},
	}
	cmd.Flags().StringVar(&o.ProjectNames, "projectNames", "", "filters by projectNames, example: test-project1,test-project2")
	cmd.Flags().StringVarP(&o.Output, "output", "o", "", "output format (options: yaml / json)")
	return cmd
}

func (o *OptionsPipelineGet) Complete(cmd *cobra.Command) error {
	var err error
	err = o.GetOptionsCommon()
	return err
}

func (o *OptionsPipelineGet) Validate(args []string) error {
	var err error
	pipelineNames := args
	for _, s := range pipelineNames {
		s = strings.Trim(s, " ")
		err = pkg.ValidateMinusNameID(s)
		if err != nil {
			err = fmt.Errorf("pipelineNames error: %s", err.Error())
			return err
		}
		o.Param.PipelineNames = append(o.Param.PipelineNames, s)
	}

	o.ProjectNames = strings.Trim(o.ProjectNames, " ")
	if o.ProjectNames != "" {
		arr := strings.Split(o.ProjectNames, ",")
		for _, s := range arr {
			s = strings.Trim(s, " ")
			err = pkg.ValidateMinusNameID(s)
			if err != nil {
				err = fmt.Errorf("--projectNames error: %s", err.Error())
				return err
			}
			o.Param.ProjectNames = append(o.Param.ProjectNames, s)
		}
	}
	if o.Output != "" {
		if o.Output != "yaml" && o.Output != "json" {
			err = fmt.Errorf("--output must be yaml or json")
			return err
		}
	}
	return err
}

func (o *OptionsPipelineGet) Run(args []string) error {
	var err error

	bs, _ := yaml.Marshal(o)
	log.Debug(fmt.Sprintf("command options:\n%s", string(bs)))

	param := map[string]interface{}{
		"projectNames": o.Param.ProjectNames,
		"page":         1,
		"perPage":      1000,
	}
	result, _, err := o.QueryAPI("api/cicd/projects", http.MethodPost, "", param, false)
	if err != nil {
		return err
	}
	rs := result.Get("data.projects").Array()
	pipelines := []pkg.Pipeline{}
	for _, r := range rs {
		project := pkg.Project{}
		err = json.Unmarshal([]byte(r.Raw), &project)
		if err != nil {
			return err
		}
		for _, pipeline := range project.Pipelines {
			pipelines = append(pipelines, pipeline)
		}
	}

	if len(pipelines) > 0 {
		if len(o.Param.PipelineNames) > 0 {
			pls := pipelines
			pipelines = []pkg.Pipeline{}
			for _, pipelineName := range o.Param.PipelineNames {
				for _, pl := range pls {
					if pl.PipelineName == pipelineName {
						pipelines = append(pipelines, pl)
						break
					}
				}
			}
		}

		dataOutput := map[string]interface{}{}
		if len(o.Param.PipelineNames) == 1 && len(pipelines) == 1 && o.Param.PipelineNames[0] == pipelines[0].PipelineName {
			dataOutput["pipeline"] = pipelines[0]
		} else {
			dataOutput["pipelines"] = pipelines
		}
		switch o.Output {
		case "json":
			bs, _ = json.MarshalIndent(dataOutput, "", "  ")
			fmt.Println(string(bs))
		case "yaml":
			s, _ := pkg.YamlIndent(dataOutput, 4)
			fmt.Println(s)
		default:
			data := [][]string{}
			for _, pipeline := range pipelines {
				pipelineName := pipeline.PipelineName
				branchName := pipeline.BranchName
				envs := strings.Join(pipeline.Envs, ",")
				envProds := strings.Join(pipeline.EnvProductions, ",")
				successCount := fmt.Sprintf("%d", pipeline.SuccessCount)
				failCount := fmt.Sprintf("%d", pipeline.FailCount)
				abortCount := fmt.Sprintf("%d", pipeline.AbortCount)
				var statusResult string
				if pipeline.Status.StartTime != "" {
					statusResult = pipeline.Status.StartTime
					if pipeline.Status.Result != "" {
						statusResult = fmt.Sprintf("%s [%s]", statusResult, pipeline.Status.Result)
					}
				}
				data = append(data, []string{pipelineName, branchName, envs, envProds, successCount, failCount, abortCount, statusResult})
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Branch", "Envs", "EnvProds", "Success", "Fail", "Abort", "LastRun"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(true)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(false)
			table.SetTablePadding("\t")
			table.SetNoWhiteSpace(true)
			table.AppendBulk(data)
			table.Render()
		}
	}

	return err
}
