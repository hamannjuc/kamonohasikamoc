package cmd

import (
	"fmt"
	"github.com/dorystack/doryctl/pkg"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

type OptionsInstallScript struct {
	*OptionsCommon
	FileName string
	Stdin    []byte
}

func NewOptionsInstallScript() *OptionsInstallScript {
	var o OptionsInstallScript
	o.OptionsCommon = OptCommon
	return &o
}

func NewCmdInstallScript() *cobra.Command {
	o := NewOptionsInstallScript()

	msgUse := fmt.Sprintf("script")
	msgShort := fmt.Sprintf("create dory-core install scripts and config files")
	msgLong := fmt.Sprintf(`create dory-core install scripts and config files, run the scripts by manual`)
	msgExample := fmt.Sprintf(`  # create dory-core install scripts and config files with docker-compose or kubernetes
  doryctl install script -f install-config.yaml
  or
  cat install-config.yaml | doryctl install script -f -
`)

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
	cmd.Flags().StringVarP(&o.FileName, "file", "f", "", "install settings YAML file")
	return cmd
}

func (o *OptionsInstallScript) Complete(cmd *cobra.Command) error {
	var err error
	return err
}

func (o *OptionsInstallScript) Validate(args []string) error {
	var err error
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
			err = fmt.Errorf("[ERROR] -f - required os.stdin\n example: echo 'xxx' | %s install script -f -", pkg.BaseCmdName)
			return err
		}
	}
	return err
}

// Run executes the appropriate steps to run a model's documentation
func (o *OptionsInstallScript) Run(args []string) error {
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
			err = fmt.Errorf("install script error: %s", err.Error())
			return err
		}
	}

	var installConfig pkg.InstallConfig
	err = yaml.Unmarshal(bs, &installConfig)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}
	validate := validator.New()
	err = validate.Struct(installConfig)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	err = installConfig.VerifyInstallConfig()
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	if installConfig.InstallMode == "docker" {
		err = o.ScriptWithDocker(installConfig)
		if err != nil {
			return err
		}
	} else if installConfig.InstallMode == "kubernetes" {
		err = o.ScriptWithKubernetes(installConfig)
		if err != nil {
			return err
		}
	} else {
		err = fmt.Errorf("install script error: installMode not correct, must be docker or kubernetes")
		return err
	}
	return err
}

func (o *OptionsInstallScript) HarborGetDockerImages() (pkg.InstallDockerImages, error) {
	var err error
	var bs []byte
	var dockerImages pkg.InstallDockerImages
	// get pull docker images
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/harbor/docker-images.yaml", pkg.DirInstallScripts))
	if err != nil {
		err = fmt.Errorf("get pull docker images error: %s", err.Error())
		return dockerImages, err
	}
	err = yaml.Unmarshal(bs, &dockerImages)
	if err != nil {
		err = fmt.Errorf("get pull docker images error: %s", err.Error())
		return dockerImages, err
	}
	return dockerImages, err
}

func (o *OptionsInstallScript) DoryCreateConfig(installConfig pkg.InstallConfig, rootDir string) error {
	var err error
	var bs []byte

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	dorycoreDir := fmt.Sprintf("%s/%s/dory-core", rootDir, installConfig.Dory.Namespace)
	dorycoreConfigDir := fmt.Sprintf("%s/config", dorycoreDir)
	dorycoreScriptDir := "dory/dory-core"
	dorycoreConfigName := "config.yaml"
	dorycoreEnvK8sName := "env-k8s-test.yaml"
	_ = os.RemoveAll(dorycoreConfigDir)
	_ = os.MkdirAll(dorycoreConfigDir, 0700)
	_ = os.MkdirAll(fmt.Sprintf("%s/dory-data", dorycoreDir), 0700)
	_ = os.MkdirAll(fmt.Sprintf("%s/tmp", dorycoreDir), 0700)

	// create config.yaml
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dorycoreScriptDir, dorycoreConfigName))
	if err != nil {
		err = fmt.Errorf("create dory-core config files error: %s", err.Error())
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
		err = fmt.Errorf("create dory-core config files error: %s", err.Error())
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

	return err
}

func (o *OptionsInstallScript) DoryCreateDockerCertsConfig(installConfig pkg.InstallConfig, rootDir string) error {
	var err error
	var bs []byte

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	dockerDir := fmt.Sprintf("%s/%s/%s", rootDir, installConfig.Dory.Namespace, installConfig.Dory.Docker.DockerName)
	_ = os.RemoveAll(dockerDir)
	_ = os.MkdirAll(dockerDir, 0700)
	dockerScriptDir := "dory/docker"
	dockerScriptName := "docker_certs.sh"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerScriptDir, dockerScriptName))
	if err != nil {
		err = fmt.Errorf("create docker certificates error: %s", err.Error())
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

	dockerDaemonJsonName := "daemon.json"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerScriptDir, dockerDaemonJsonName))
	if err != nil {
		err = fmt.Errorf("create docker config error: %s", err.Error())
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
		err = fmt.Errorf("create docker config files error: %s", err.Error())
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

	return err
}

func (o *OptionsInstallScript) DoryCreateKubernetesDataPod(installConfig pkg.InstallConfig, rootDir string) error {
	var err error
	var bs []byte

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	kubernetesDir := "kubernetes"
	projectDataAlpineName := "project-data-alpine.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, kubernetesDir, projectDataAlpineName))
	if err != nil {
		err = fmt.Errorf("create project-data-alpine in kubernetes error: %s", err.Error())
		return err
	}
	strProjectDataAlpine, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create project-data-alpine in kubernetes error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", rootDir, projectDataAlpineName), []byte(strProjectDataAlpine), 0600)
	if err != nil {
		err = fmt.Errorf("create project-data-alpine in kubernetes error: %s", err.Error())
		return err
	}

	return err
}

func (o *OptionsInstallScript) DoryCreateInstallReadme(installConfig pkg.InstallConfig, readmeInstallDir, doryInstallSettingsName string) error {
	var err error
	var bs []byte

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s", pkg.DirInstallScripts, doryInstallSettingsName))
	if err != nil {
		err = fmt.Errorf("create dory install settings readme error: %s", err.Error())
		return err
	}
	strDoryInstallSettings, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory install settings readme error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/README-%s.md", readmeInstallDir, installConfig.InstallMode), []byte(strDoryInstallSettings), 0600)
	if err != nil {
		err = fmt.Errorf("create dory install settings readme error: %s", err.Error())
		return err
	}
	LogWarning(fmt.Sprintf("####################################################"))
	LogWarning(fmt.Sprintf("PLEASE FOLLOW THE INSTRUCTION TO FINISH DORY INSTALL"))
	LogWarning(fmt.Sprintf("README located at: %s/README-%s.md", readmeInstallDir, installConfig.InstallMode))
	LogWarning(fmt.Sprintf("\n\n%s", strDoryInstallSettings))

	return err
}

func (o *OptionsInstallScript) DoryCreateResetReadme(installConfig pkg.InstallConfig, readmeResetDir, doryResetSettingsName string) error {
	var err error
	var bs []byte

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s", pkg.DirInstallScripts, doryResetSettingsName))
	if err != nil {
		err = fmt.Errorf("create dory reset settings readme error: %s", err.Error())
		return err
	}
	strDoryResetSettings, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory reset settings readme error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/README-reset-%s.md", readmeResetDir, installConfig.InstallMode), []byte(strDoryResetSettings), 0600)
	if err != nil {
		err = fmt.Errorf("create dory reset settings readme error: %s", err.Error())
		return err
	}
	LogWarning(fmt.Sprintf("####################################################"))
	LogWarning(fmt.Sprintf("PLEASE FOLLOW THE INSTRUCTION TO REMOVE DORY INSTALL"))
	LogWarning(fmt.Sprintf("README.md located at: %s/README-reset-%s.md", readmeResetDir, installConfig.InstallMode))
	LogWarning(fmt.Sprintf("\n\n%s", strDoryResetSettings))

	return err
}

func (o *OptionsInstallScript) ScriptWithDocker(installConfig pkg.InstallConfig) error {
	var err error
	bs := []byte{}

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	dockerInstallDir := "dory-install-docker"
	_ = os.RemoveAll(dockerInstallDir)
	_ = os.MkdirAll(dockerInstallDir, 0700)

	doryResetDockerSettingsName := "dory-reset-docker-settings.md"
	defer o.DoryCreateResetReadme(installConfig, dockerInstallDir, doryResetDockerSettingsName)

	// create harbor certificates
	harborDir := fmt.Sprintf("%s/%s", dockerInstallDir, installConfig.ImageRepo.Namespace)
	_ = os.RemoveAll(harborDir)
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

	// extract harbor install files
	err = pkg.ExtractEmbedFile(pkg.FsInstallScripts, fmt.Sprintf("%s/harbor/harbor", pkg.DirInstallScripts), harborDir)
	if err != nil {
		err = fmt.Errorf("extract harbor install files error: %s", err.Error())
		return err
	}

	harborInstallerDir := "harbor/harbor"
	harborYamlName := "harbor.yml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborInstallerDir, harborYamlName))
	if err != nil {
		err = fmt.Errorf("create harbor.yml error: %s", err.Error())
		return err
	}
	strHarborYaml, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create harbor.yml error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", harborDir, harborYamlName), []byte(strHarborYaml), 0600)
	if err != nil {
		err = fmt.Errorf("create harbor.yml error: %s", err.Error())
		return err
	}

	harborPrepareName := "prepare"
	_ = os.Rename(fmt.Sprintf("%s/harbor", installConfig.RootDir), harborDir)
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborInstallerDir, harborPrepareName))
	if err != nil {
		err = fmt.Errorf("create prepare error: %s", err.Error())
		return err
	}
	strHarborPrepare, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create prepare error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", harborDir, harborPrepareName), []byte(strHarborPrepare), 0700)
	if err != nil {
		err = fmt.Errorf("create prepare error: %s", err.Error())
		return err
	}

	_ = os.Chmod(fmt.Sprintf("%s/common.sh", harborDir), 0700)
	_ = os.Chmod(fmt.Sprintf("%s/install.sh", harborDir), 0700)
	_ = os.Chmod(fmt.Sprintf("%s/prepare", harborDir), 0700)

	////////////////////////////////////////////////////

	// create dory docker-compose.yaml
	doryDir := fmt.Sprintf("%s/%s", dockerInstallDir, installConfig.Dory.Namespace)
	_ = os.RemoveAll(doryDir)
	_ = os.MkdirAll(doryDir, 0700)
	dockerComposeDir := "dory"
	dockerComposeName := "docker-compose.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, dockerComposeDir, dockerComposeName))
	if err != nil {
		err = fmt.Errorf("create dory docker-compose.yaml error: %s", err.Error())
		return err
	}
	strCompose, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory docker-compose.yaml error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", doryDir, dockerComposeName), []byte(strCompose), 0600)
	if err != nil {
		err = fmt.Errorf("create dory docker-compose.yaml error: %s", err.Error())
		return err
	}

	// create dory-core config files
	err = o.DoryCreateConfig(installConfig, dockerInstallDir)
	if err != nil {
		return err
	}

	// create docker certificates and config
	err = o.DoryCreateDockerCertsConfig(installConfig, dockerInstallDir)
	if err != nil {
		return err
	}

	//////////////////////////////////////////////////

	// create project-data-alpine in kubernetes
	err = o.DoryCreateKubernetesDataPod(installConfig, dockerInstallDir)
	if err != nil {
		return err
	}

	//////////////////////////////////////////////////

	// create dory install docker settings readme
	doryInstallDockerSettingsName := "dory-install-docker-settings.md"
	err = o.DoryCreateInstallReadme(installConfig, dockerInstallDir, doryInstallDockerSettingsName)
	if err != nil {
		return err
	}

	return err
}

func (o *OptionsInstallScript) ScriptWithKubernetes(installConfig pkg.InstallConfig) error {
	var err error
	bs := []byte{}

	bs, _ = yaml.Marshal(installConfig)
	vals := map[string]interface{}{}
	err = yaml.Unmarshal(bs, &vals)
	if err != nil {
		err = fmt.Errorf("install script error: %s", err.Error())
		return err
	}

	kubernetesInstallDir := "dory-install-kubernetes"
	_ = os.RemoveAll(kubernetesInstallDir)
	_ = os.MkdirAll(kubernetesInstallDir, 0700)

	doryResetKubernetesSettingsName := "dory-reset-kubernetes-settings.md"
	defer o.DoryCreateResetReadme(installConfig, kubernetesInstallDir, doryResetKubernetesSettingsName)

	harborInstallerDir := "kubernetes/harbor"
	harborInstallYamlDir := fmt.Sprintf("%s/harbor", kubernetesInstallDir)
	_ = os.RemoveAll(harborInstallYamlDir)
	_ = os.MkdirAll(harborInstallYamlDir, 0700)

	// extract harbor helm files
	err = pkg.ExtractEmbedFile(pkg.FsInstallScripts, fmt.Sprintf("%s/%s", pkg.DirInstallScripts, harborInstallerDir), harborInstallYamlDir)
	if err != nil {
		err = fmt.Errorf("extract harbor helm files error: %s", err.Error())
		return err
	}

	harborValuesYamlName := "values.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/%s/%s", pkg.DirInstallScripts, harborInstallerDir, harborValuesYamlName))
	if err != nil {
		err = fmt.Errorf("create values.yaml error: %s", err.Error())
		return err
	}
	strHarborValuesYaml, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create values.yaml error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", harborInstallYamlDir, harborValuesYamlName), []byte(strHarborValuesYaml), 0600)
	if err != nil {
		err = fmt.Errorf("create values.yaml error: %s", err.Error())
		return err
	}

	// create harbor namespace and pv pvc
	vals["currentNamespace"] = installConfig.ImageRepo.Namespace
	step01NamespacePvName := "step01-namespace-pv.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, step01NamespacePvName))
	if err != nil {
		err = fmt.Errorf("create harbor namespace and pv pvc error: %s", err.Error())
		return err
	}
	strStep01NamespacePv, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create harbor namespace and pv pvc error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", kubernetesInstallDir, step01NamespacePvName), []byte(strStep01NamespacePv), 0600)
	if err != nil {
		err = fmt.Errorf("create harbor namespace and pv pvc error: %s", err.Error())
		return err
	}

	harborUpdateCertsName := "harbor_update_docker_certs.sh"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, harborUpdateCertsName))
	if err != nil {
		err = fmt.Errorf("update docker harbor certificates error: %s", err.Error())
		return err
	}
	strHarborUpdateCerts, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("update docker harbor certificates error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", kubernetesInstallDir, harborUpdateCertsName), []byte(strHarborUpdateCerts), 0600)
	if err != nil {
		err = fmt.Errorf("update docker harbor certificates error: %s", err.Error())
		return err
	}

	//////////////////////////////////////////////////

	// create dory namespace and pv pvc
	doryInstallDir := fmt.Sprintf("%s/%s", kubernetesInstallDir, installConfig.Dory.Namespace)
	_ = os.MkdirAll(doryInstallDir, 0700)
	vals["currentNamespace"] = installConfig.Dory.Namespace
	step01NamespacePvName = "step01-namespace-pv.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, step01NamespacePvName))
	if err != nil {
		err = fmt.Errorf("create dory namespace and pv pvc error: %s", err.Error())
		return err
	}
	strStep01NamespacePv, err = pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory namespace and pv pvc error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", doryInstallDir, step01NamespacePvName), []byte(strStep01NamespacePv), 0600)
	if err != nil {
		err = fmt.Errorf("create dory namespace and pv pvc error: %s", err.Error())
		return err
	}

	// create dory install yaml
	doryInstallYamlName := "dory-install.yaml"
	step02StatefulsetName := "step02-statefulset.yaml"
	step03ServiceName := "step03-service.yaml"
	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, doryInstallYamlName))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	strDoryInstallYaml, err := pkg.ParseTplFromVals(vals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	var installVals map[string]interface{}
	_ = yaml.Unmarshal([]byte(strDoryInstallYaml), &installVals)
	for k, v := range vals {
		installVals[k] = v
	}

	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, step02StatefulsetName))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	strStep02Statefulset, err := pkg.ParseTplFromVals(installVals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", doryInstallDir, step02StatefulsetName), []byte(strStep02Statefulset), 0600)
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}

	bs, err = pkg.FsInstallScripts.ReadFile(fmt.Sprintf("%s/kubernetes/%s", pkg.DirInstallScripts, step03ServiceName))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	strStep03Service, err := pkg.ParseTplFromVals(installVals, string(bs))
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", doryInstallDir, step03ServiceName), []byte(strStep03Service), 0600)
	if err != nil {
		err = fmt.Errorf("create dory install yaml error: %s", err.Error())
		return err
	}

	// create dory-core config files
	err = o.DoryCreateConfig(installConfig, kubernetesInstallDir)
	if err != nil {
		return err
	}

	// create docker certificates and config
	err = o.DoryCreateDockerCertsConfig(installConfig, kubernetesInstallDir)
	if err != nil {
		return err
	}
	//dockerDir := fmt.Sprintf("%s/%s/%s", installConfig.RootDir, installConfig.Dory.Namespace, installConfig.Dory.Docker.DockerName)
	//
	//// put docker certificates in kubernetes
	//LogInfo("put docker certificates in kubernetes begin")
	//cmdSecret := fmt.Sprintf(`kubectl -n %s create secret generic %s-tls --from-file=certs/ca.crt --from-file=certs/tls.crt --from-file=certs/tls.key --dry-run=client -o yaml | kubectl apply -f -`, installConfig.Dory.Namespace, installConfig.Dory.Docker.DockerName)
	//_, _, err = pkg.CommandExec(cmdSecret, dockerDir)
	//if err != nil {
	//	err = fmt.Errorf("put docker certificates in kubernetes error: %s", err.Error())
	//	return err
	//}
	//dockerScriptName := "docker_certs.sh"
	//_ = os.RemoveAll(fmt.Sprintf("%s/%s", dockerDir, dockerScriptName))
	//_ = os.RemoveAll(fmt.Sprintf("%s/certs", dockerDir))
	//LogSuccess(fmt.Sprintf("put docker certificates in kubernetes success"))
	//
	//// put harbor certificates in docker directory
	//LogInfo("put harbor certificates in docker directory begin")
	//_ = os.RemoveAll(fmt.Sprintf("%s/%s", dockerDir, installConfig.ImageRepo.DomainName))
	//_, _, err = pkg.CommandExec(fmt.Sprintf("cp -r /etc/docker/certs.d/%s %s", installConfig.ImageRepo.DomainName, dockerDir), dockerDir)
	//if err != nil {
	//	err = fmt.Errorf("put harbor certificates in docker directory error: %s", err.Error())
	//	return err
	//}
	//LogSuccess(fmt.Sprintf("put harbor certificates in docker directory success"))
	//
	//// create directories and nexus data
	//err = o.DoryCreateDirs(installConfig)
	//if err != nil {
	//	return err
	//}
	//
	//// deploy all dory services in kubernetes
	//LogInfo("deploy all dory services in kubernetes begin")
	//_, _, err = pkg.CommandExec(fmt.Sprintf("kubectl apply -f %s", step02StatefulsetName), kubernetesInstallDir)
	//if err != nil {
	//	err = fmt.Errorf("deploy all dory services in kubernetes error: %s", err.Error())
	//	return err
	//}
	//_, _, err = pkg.CommandExec(fmt.Sprintf("kubectl apply -f %s", step03ServiceName), kubernetesInstallDir)
	//if err != nil {
	//	err = fmt.Errorf("deploy all dory services in kubernetes error: %s", err.Error())
	//	return err
	//}
	//LogSuccess(fmt.Sprintf("deploy all dory services in kubernetes success"))
	//
	//// waiting for dory to ready
	//err = o.KubernetesCheckPodStatus(installConfig, "dory")
	//if err != nil {
	//	return err
	//}
	//
	////////////////////////////////////////////////////
	//
	//// create project-data-alpine in kubernetes
	//err = o.DoryCreateKubernetesDataPod(installConfig)
	//if err != nil {
	//	return err
	//}
	//
	////////////////////////////////////////////////////
	//
	//// create dory install kubernetes settings readme
	//doryInstallKubernetesSettingsName := "dory-install-kubernetes-settings.md"
	//err = o.DoryCreateInstallReadme(installConfig, kubernetesInstallDir, doryInstallKubernetesSettingsName)
	//if err != nil {
	//	return err
	//}

	return err
}