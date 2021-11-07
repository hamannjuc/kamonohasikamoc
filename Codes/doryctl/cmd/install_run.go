package cmd

import (
	"fmt"
	"github.com/dorystack/doryctl/pkg"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
	"time"
)

type OptionsInstallRun struct {
	*OptionsCommon
	Mode     string
	FileName string
	Stdin    []byte
}

func NewOptionsInstallRun() *OptionsInstallRun {
	var o OptionsInstallRun
	o.OptionsCommon = OptCommon
	return &o
}

func NewCmdInstallRun() *cobra.Command {
	o := NewOptionsInstallRun()

	msgUse := fmt.Sprintf("run")
	msgShort := fmt.Sprintf("run install dory-core with docker or kubernetes")
	msgLong := fmt.Sprintf(`run install dory-core and relative components with docker-compose or kubernetes`)
	msgExample := fmt.Sprintf(`# run install dory-core and relative components with docker-compose, will create all config files and docker-compose.yaml file
%s install run --mode docker -f docker.yaml

#  run install dory-core and relative components with kubernetes, will create all config files and kubernetes deploy YAML files
%s install run --mode kubernetes -f kubernetes.yaml
`, pkg.BaseCmdName, pkg.BaseCmdName)

	cmd := &cobra.Command{
		Use:                   msgUse,
		DisableFlagsInUseLine: true,
		Short:                 msgShort,
		Long:                  msgLong,
		Example:               msgExample,
		Run: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(o.Complete(cmd))
			cobra.CheckErr(o.Validate(args))
			cobra.CheckErr(o.Run(args))
		},
	}
	cmd.Flags().StringVar(&o.Mode, "mode", "", "install mode, options: docker, kubernetes")
	cmd.Flags().StringVarP(&o.FileName, "file", "f", "", "install settings YAML file")
	return cmd
}

func (o *OptionsInstallRun) Complete(cmd *cobra.Command) error {
	var err error
	return err
}

func (o *OptionsInstallRun) Validate(args []string) error {
	var err error
	if o.Mode != "docker" && o.Mode != "kubernetes" {
		err = fmt.Errorf("[ERROR] --mode must be docker or kubernetes")
		return err
	}
	if o.FileName == "" {
		err = fmt.Errorf("[ERROR] -f required")
		return err
	}
	if o.FileName == "-" {
		bs, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		o.Stdin = bs
		if len(o.Stdin) == 0 {
			err = fmt.Errorf("[ERROR] -f - required os.stdin\n example: echo 'xxx' | %s install run --mode %s --f -", pkg.BaseCmdName, o.Mode)
			return err
		}
	}
	return err
}

// Run executes the appropriate steps to run a model's documentation
func (o *OptionsInstallRun) Run(args []string) error {
	var err error

	bs := []byte{}

	defer func() {
		if err != nil {
			LogError(err.Error())
		}
	}()

	if o.FileName == "-" {
		bs = o.Stdin
	} else {
		bs, err = os.ReadFile(o.FileName)
		if err != nil {
			err = fmt.Errorf("install run error: %s", err.Error())
			return err
		}
	}

	if o.Mode == "docker" {
		var installDockerConfig pkg.InstallDockerConfig
		err = yaml.Unmarshal(bs, &installDockerConfig)
		if err != nil {
			err = fmt.Errorf("install run error: %s", err.Error())
			return err
		}
		validate := validator.New()
		err = validate.Struct(installDockerConfig)
		if err != nil {
			err = fmt.Errorf("install run error: %s", err.Error())
			return err
		}

		err = installDockerConfig.VerifyInstallDockerConfig()
		if err != nil {
			err = fmt.Errorf("install run error: %s", err.Error())
			return err
		}
		bs, _ = yaml.Marshal(installDockerConfig)

		vals := map[string]interface{}{}
		err = yaml.Unmarshal(bs, &vals)
		if err != nil {
			err = fmt.Errorf("install run error: %s", err.Error())
			return err
		}

		// create harbor certificates
		harborDir := fmt.Sprintf("%s/%s", installDockerConfig.RootDir, installDockerConfig.HarborDir)
		_ = os.MkdirAll(harborDir, 0700)
		harborScriptDir := "harbor"
		harborScriptName := "harbor_certs.sh"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborScriptDir, harborScriptName))
		if err != nil {
			err = fmt.Errorf("create harbor certificates error: %s", err.Error())
			return err
		}
		strHarborCertScript, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create harbor certificates error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", harborDir, harborScriptName), []byte(strHarborCertScript), 0600)
		if err != nil {
			err = fmt.Errorf("create harbor certificates error: %s", err.Error())
			return err
		}

		LogInfo("create harbor certificates begin")
		_, _, err = pkg.CommandExec(fmt.Sprintf("sh %s", harborScriptName), harborDir)
		if err != nil {
			err = fmt.Errorf("create harbor certificates error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create harbor certificates %s/%s success", harborDir, installDockerConfig.Harbor.CertsDir))

		// pull docker images
		harborDockerImagesYaml := "docker-images.yaml"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborScriptDir, harborDockerImagesYaml))
		if err != nil {
			err = fmt.Errorf("pull docker images error: %s", err.Error())
			return err
		}
		var dockerImages pkg.InstallDockerImages
		err = yaml.Unmarshal(bs, &dockerImages)
		if err != nil {
			err = fmt.Errorf("pull docker images error: %s", err.Error())
			return err
		}
		LogInfo("docker images need to pull")
		for _, idi := range dockerImages.InstallDockerImages {
			fmt.Println(fmt.Sprintf("%s", idi.Source))
		}
		//LogInfo("pull docker images begin")
		//for _, idi := range dockerImages.InstallDockerImages {
		//	_, _, err = pkg.CommandExec(fmt.Sprintf("docker pull %s", idi.Source), ".")
		//	if err != nil {
		//		err = fmt.Errorf("pull docker image %s error: %s", idi.Source, err.Error())
		//		return err
		//	}
		//}
		//LogSuccess(fmt.Sprintf("pull docker images success"))

		// extract harbor install files
		err = pkg.ExtractEmbedFile(pkg.FsInstallScripts, fmt.Sprintf("%s/harbor/harbor", pkg.DirInstallScripts), harborDir)
		if err != nil {
			err = fmt.Errorf("extract harbor install files error: %s", err.Error())
			return err
		}

		harborYamlDir := "harbor/harbor"
		harborYamlName := "harbor.yml"
		_ = os.Rename(fmt.Sprintf("%s/harbor", installDockerConfig.RootDir), harborDir)
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborYamlDir, harborYamlName))
		if err != nil {
			err = fmt.Errorf("create create harbor.yml error: %s", err.Error())
			return err
		}
		strHarborYaml, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create create harbor.yml error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", harborDir, harborYamlName), []byte(strHarborYaml), 0600)
		if err != nil {
			err = fmt.Errorf("create create harbor.yml error: %s", err.Error())
			return err
		}
		_ = os.Chmod(fmt.Sprintf("%s/common.sh", harborDir), 0700)
		_ = os.Chmod(fmt.Sprintf("%s/install.sh", harborDir), 0700)
		_ = os.Chmod(fmt.Sprintf("%s/prepare", harborDir), 0700)
		LogSuccess(fmt.Sprintf("create %s/%s success", harborDir, harborYamlName))
		LogSuccess(fmt.Sprintf("extract harbor install files %s success", harborDir))

		// install harbor
		LogInfo("install harbor begin")
		_, _, err = pkg.CommandExec(fmt.Sprintf("./install.sh"), harborDir)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		_, _, err = pkg.CommandExec(fmt.Sprintf("sleep 5 && docker-compose stop && docker-compose rm -f"), harborDir)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		bs, err = os.ReadFile(fmt.Sprintf("%s/docker-compose.yml", harborDir))
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		strHarborComposeYaml := strings.Replace(string(bs), harborDir, ".", -1)
		err = os.WriteFile(fmt.Sprintf("%s/docker-compose.yml", harborDir), []byte(strHarborComposeYaml), 0600)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		// update /etc/hosts
		_, _, err = pkg.CommandExec(fmt.Sprintf("cat /etc/hosts | grep %s", installDockerConfig.Harbor.DomainName), harborDir)
		if err != nil {
			// harbor domain name not exists
			_, _, err = pkg.CommandExec(fmt.Sprintf("sudo echo '%s  %s' >> /etc/hosts", installDockerConfig.HostIP, installDockerConfig.Harbor.DomainName), harborDir)
			if err != nil {
				err = fmt.Errorf("install harbor error: %s", err.Error())
				return err
			}
			LogInfo("add harbor domain name to /etc/hosts")
		}
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker-compose up -d"), harborDir)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		LogInfo("waiting harbor boot up for 10 seconds")
		time.Sleep(time.Second * 10)
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker-compose ps"), harborDir)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		LogInfo("docker login to harbor")
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker login --username admin --password %s %s", installDockerConfig.Harbor.Password, installDockerConfig.Harbor.DomainName), harborDir)
		if err != nil {
			err = fmt.Errorf("install harbor error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("install harbor at %s success", harborDir))

		// create harbor project public, hub, gcr, quay
		LogInfo("create harbor project public, hub, gcr, quay begin")
		harborCreateProjectCmd := fmt.Sprintf(`curl -k -X POST "https://admin:%s@%s/api/v2.0/projects" -H  "accept: application/json" -H  "Content-Type: application/json" -d '{"project_name": "public", "public": true}' && \
			curl -k -X POST "https://admin:%s@%s/api/v2.0/projects" -H  "accept: application/json" -H  "Content-Type: application/json" -d '{"project_name": "hub", "public": true}' && \
			curl -k -X POST "https://admin:%s@%s/api/v2.0/projects" -H  "accept: application/json" -H  "Content-Type: application/json" -d '{"project_name": "gcr", "public": true}' && \
			curl -k -X POST "https://admin:%s@%s/api/v2.0/projects" -H  "accept: application/json" -H  "Content-Type: application/json" -d '{"project_name": "quay", "public": true}'`, installDockerConfig.Harbor.Password, installDockerConfig.Harbor.DomainName, installDockerConfig.Harbor.Password, installDockerConfig.Harbor.DomainName, installDockerConfig.Harbor.Password, installDockerConfig.Harbor.DomainName, installDockerConfig.Harbor.Password, installDockerConfig.Harbor.DomainName)
		_, _, err = pkg.CommandExec(harborCreateProjectCmd, harborDir)
		if err != nil {
			err = fmt.Errorf("create harbor project public, hub, gcr, quay error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("install harbor at %s success", harborDir))

		LogInfo("docker images push to harbor begin")
		for i, idi := range dockerImages.InstallDockerImages {
			targetImage := fmt.Sprintf("%s/%s", installDockerConfig.Harbor.DomainName, idi.Target)
			_, _, err = pkg.CommandExec(fmt.Sprintf("docker tag %s %s && docker push %s", idi.Source, targetImage, targetImage), ".")
			if err != nil {
				err = fmt.Errorf("docker images push to harbor %s error: %s", idi.Source, err.Error())
				return err
			}
			fmt.Println(fmt.Sprintf("# progress: %d/%d", i+1, len(dockerImages.InstallDockerImages)))
		}
		LogSuccess(fmt.Sprintf("docker images push to harbor success"))

		//////////////////////////////////////////////////

		// create dory docker-compose.yaml
		doryDir := fmt.Sprintf("%s/%s", installDockerConfig.RootDir, installDockerConfig.DoryDir)
		dorycoreDir := fmt.Sprintf("%s/%s/dory-core", installDockerConfig.RootDir, installDockerConfig.DoryDir)
		_ = os.MkdirAll(fmt.Sprintf("%s/dory-data", dorycoreDir), 0700)
		_ = os.MkdirAll(fmt.Sprintf("%s/tmp", dorycoreDir), 0700)
		dockerComposeDir := "dory"
		dockerComposeName := "docker-compose.yaml"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerComposeDir, dockerComposeName))
		if err != nil {
			err = fmt.Errorf("create create dory docker-compose.yaml error: %s", err.Error())
			return err
		}
		strCompose, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create create dory docker-compose.yaml error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", doryDir, dockerComposeName), []byte(strCompose), 0600)
		if err != nil {
			err = fmt.Errorf("create create dory docker-compose.yaml error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create %s/%s success", doryDir, dockerComposeName))

		// create dory-core config files
		dorycoreConfigDir := fmt.Sprintf("%s/config", dorycoreDir)
		dorycoreScriptDir := "dory/dory-core"
		dorycoreConfigName := "config.yaml"
		dorycoreEnvK8sName := "env-k8s-test.yaml"
		_ = os.MkdirAll(dorycoreConfigDir, 0700)
		// create config.yaml
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dorycoreScriptDir, dorycoreConfigName))
		if err != nil {
			return err
		}
		strDorycoreConfig, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create dory-core config files error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dorycoreConfigDir, dorycoreConfigName), []byte(strDorycoreConfig), 0600)
		if err != nil {
			err = fmt.Errorf("create dory-core config files error: %s", err.Error())
			return err
		}
		// create env-k8s-test.yaml
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dorycoreScriptDir, dorycoreEnvK8sName))
		if err != nil {
			return err
		}
		strDorycoreEnvK8s, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create dory-core config files error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dorycoreConfigDir, dorycoreEnvK8sName), []byte(strDorycoreEnvK8s), 0600)
		if err != nil {
			err = fmt.Errorf("create dory-core config files error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create dory-core config files %s success", dorycoreConfigDir))

		// create docker certificates
		dockerDir := fmt.Sprintf("%s/%s/%s", installDockerConfig.RootDir, installDockerConfig.DoryDir, installDockerConfig.Dory.Docker.DockerName)
		_ = os.MkdirAll(dockerDir, 0700)
		dockerScriptDir := "dory/docker"
		dockerScriptName := "docker_certs.sh"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerScriptDir, dockerScriptName))
		if err != nil {
			return err
		}
		strDockerCertScript, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create docker certificates error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dockerDir, dockerScriptName), []byte(strDockerCertScript), 0600)
		if err != nil {
			err = fmt.Errorf("create docker certificates error: %s", err.Error())
			return err
		}

		LogInfo("create docker certificates begin")
		_, _, err = pkg.CommandExec(fmt.Sprintf("sh %s", dockerScriptName), dockerDir)
		if err != nil {
			err = fmt.Errorf("create docker certificates error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create docker certificates %s/certs success", dockerDir))

		dockerDaemonJsonName := "daemon.json"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerScriptDir, dockerDaemonJsonName))
		if err != nil {
			return err
		}
		strDockerDaemonJson, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create docker config error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dockerDir, dockerDaemonJsonName), []byte(strDockerDaemonJson), 0600)
		if err != nil {
			err = fmt.Errorf("create docker config error: %s", err.Error())
			return err
		}

		dockerConfigJsonName := "config.json"
		bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerScriptDir, dockerConfigJsonName))
		if err != nil {
			return err
		}
		strDockerConfigJson, err := pkg.ParseTplFromVals(vals, string(bs))
		if err != nil {
			err = fmt.Errorf("create docker config files error: %s", err.Error())
			return err
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dockerDir, dockerConfigJsonName), []byte(strDockerConfigJson), 0600)
		if err != nil {
			err = fmt.Errorf("create docker config files error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create docker config files %s success", dockerDir))

		// get nexus init data
		_, _, err = pkg.CommandExec(fmt.Sprintf("(docker rm -f nexus-data-init || true) && docker run -d -t --name nexus-data-init dorystack/nexus-data-init:alpine-3.15.0 cat"), doryDir)
		if err != nil {
			err = fmt.Errorf("get nexus init data error: %s", err.Error())
			return err
		}
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker cp nexus-data-init:/nexus-data/nexus . && docker rm -f nexus-data-init"), doryDir)
		if err != nil {
			err = fmt.Errorf("get nexus init data error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("get nexus init data %s success", doryDir))

		// create directory and chown
		_ = os.MkdirAll(fmt.Sprintf("%s/mongo-core-dory", doryDir), 0700)
		_, _, err = pkg.CommandExec(fmt.Sprintf("sudo chown -R 999:999 %s/mongo-core-dory", doryDir), doryDir)
		if err != nil {
			err = fmt.Errorf("create directory and chown error: %s", err.Error())
			return err
		}
		_, _, err = pkg.CommandExec(fmt.Sprintf("sudo chown -R 200:200 %s/nexus", doryDir), doryDir)
		if err != nil {
			err = fmt.Errorf("create directory and chown error: %s", err.Error())
			return err
		}
		_, _, err = pkg.CommandExec(fmt.Sprintf("sudo chown -R 1000:1000 %s/dory-core", doryDir), doryDir)
		if err != nil {
			err = fmt.Errorf("create directory and chown error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("create directory and chown %s success", doryDir))

		// run all dory services
		LogInfo("run all dory services begin")
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker-compose up -d"), doryDir)
		if err != nil {
			err = fmt.Errorf("run all dory services error: %s", err.Error())
			return err
		}
		LogInfo("waiting all dory services boot up for 10 seconds")
		time.Sleep(time.Second * 10)
		_, _, err = pkg.CommandExec(fmt.Sprintf("docker-compose ps"), doryDir)
		if err != nil {
			err = fmt.Errorf("run all dory services error: %s", err.Error())
			return err
		}
		LogSuccess(fmt.Sprintf("run all dory services %s success", doryDir))

	} else if o.Mode == "kubernetes" {
		fmt.Println("args:", args)
		fmt.Println("install with kubernetes")
	}
	return err
}
