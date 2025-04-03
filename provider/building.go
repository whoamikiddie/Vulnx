package provider

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
)

func (p *Provider) PrePareBuildData() {
	contentFile := path.Join(p.Opt.Env.CloudConfigFolder, fmt.Sprintf("providers/%s.provider", p.ProviderName))
	content := utils.GetFileContent(contentFile)

	data := make(map[string]string)
	data["snapshot_name"] = p.SnapshotName
	data["api_token"] = p.Token
	// for aws only
	data["access_key"] = p.AccessKeyId
	data["secret_key"] = p.SecretKey
	data["source_ami"] = p.ProviderConfig.DefaultImage

	// c.Cloud.ProviderFolder --> ~/.osmedeus/provider/<osmp-name>-v4.x-randomstring
	p.ProviderConfig.ProviderFolder = path.Join(p.Opt.Env.ProviderFolder, fmt.Sprintf("%s-%s", p.SnapshotName, utils.RandomString(6)))
	utils.MakeDir(p.ProviderConfig.ProviderFolder)
	data["ProviderFolder"] = p.ProviderConfig.ProviderFolder

	data["image"] = p.ProviderConfig.DefaultImage
	data["size"] = p.ProviderConfig.Size
	data["region"] = p.ProviderConfig.Region
	data["TS"] = utils.GetTS()

	// generate packer content file to run
	providerString := utils.RenderText(content, data)
	data["Builder"] = providerString

	// ~/osmedeus-base
	data["BaseFolder"] = utils.NormalizePath(strings.TrimLeft(p.Opt.Env.BaseFolder, "/"))
	data["Plugins"] = p.Opt.Env.BinariesFolder
	data["OBin"] = p.Opt.Env.BinariesFolder
	data["Data"] = p.Opt.Env.DataFolder
	data["Cloud"] = p.Opt.Env.CloudConfigFolder
	data["Workflow"] = p.Opt.Env.WorkFlowsFolder

	// ~/.osmedeus/workspaces
	data["Workspaces"] = p.Opt.Env.WorkspacesFolder
	data["Binary"] = libs.BINARY
	data["VERSION"] = libs.VERSION
	data["BuildRepo"] = p.Opt.Cloud.BuildRepo

	// for terraform
	data["ssh_public_key"] = p.Opt.Cloud.PublicKeyContent
	data["root_password"] = fmt.Sprintf("%s-%s", libs.SNAPSHOT, utils.RandomString(8))

	//spew.Dump("data --> ", data)
	//spew.Dump("p.ProviderConfig --> ", p.ProviderConfig)

	p.ProviderConfig.BuildData = data
}

func (p *Provider) BuildImage() (err error) {
	if p.SnapshotFound && !p.Opt.Cloud.ReBuildBaseImage {
		return nil
	}

	p.PrePareBuildData()
	p.DeleteOldSnapshot()

	// p.ProviderConfig.ProviderFolder --> ~/.osmedeus/provider/<osmp-name>

	utils.DebugF("Cleaning old provider build: %s", p.ProviderConfig.ProviderFolder)
	os.RemoveAll(p.ProviderConfig.ProviderFolder)
	utils.MakeDir(p.ProviderConfig.ProviderFolder)

	// generate provision process
	setupContent := utils.GetFileContent(path.Join(p.Opt.Env.CloudConfigFolder, "setup.sh"))
	setupContent = utils.RenderText(setupContent, p.ProviderConfig.BuildData)
	setupFile := path.Join(p.ProviderConfig.ProviderFolder, "setup.sh")
	utils.WriteToFile(setupFile, setupContent)

	// generate build file
	var buildContent string
	buildContentFile := path.Join(p.Opt.Env.CloudConfigFolder, "general-build.packer")
	switch p.ProviderName {
	case "do", "digitalocean":
		buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "digitalocean-build.packer")
		if !utils.FileExists(buildContentFile) {
			buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "do-build.packer")
			if !utils.FileExists(buildContentFile) {
				buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "general-build.packer")
			}
		}

	case "ln", "line", "linode":
		buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "linode-build.packer")
		if !utils.FileExists(buildContentFile) {
			buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "ln-build.packer")
		}
	default:
		buildContentFile = path.Join(p.Opt.Env.CloudConfigFolder, "general-build.packer")
	}

	buildContent = utils.GetFileContent(buildContentFile)
	if buildContent == "" {
		errStr := fmt.Sprintf("Build file content not found at: %v", buildContentFile)
		utils.ErrorF(errStr)
		return fmt.Errorf(errStr)
	}

	buildContent = utils.RenderText(buildContent, p.ProviderConfig.BuildData)
	buildFile := path.Join(p.ProviderConfig.ProviderFolder, "build.json")
	p.ProviderConfig.BuildFile = buildFile
	utils.WriteToFile(buildFile, buildContent)
	utils.InforF("Write build provision of %s to: %s", color.HiYellowString(p.ProviderName), color.HiCyanString(buildFile))

	// actually run building
	err = p.Action(RunBuild)
	if err != nil {
		p.SnapshotFound = false
		return err
	}

	err = p.Action(ListImage)
	return err
}

// RunBuild run the packer command
func (p *Provider) RunBuild() error {
	packerBinary := fmt.Sprintf("%s/packer", p.Opt.Env.BinariesFolder)
	if !utils.FileExists(packerBinary) {
		packerBinary = "packer"
	}

	cmd := fmt.Sprintf("%s validate %s", packerBinary, p.ProviderConfig.BuildFile)
	out, err := utils.RunCommandWithErr(cmd)
	if err != nil {
		utils.ErrorF(out)
		return err
	}
	utils.InforF("The Packer file appears to be functioning properly: %s", color.HiCyanString(p.ProviderConfig.BuildFile))

	// really start to build stuff here
	utils.TSPrintF("Start packer build for: %s", color.HiCyanString(p.ProviderConfig.BuildFile))
	cmd = fmt.Sprintf("%s build %s", packerBinary, p.ProviderConfig.BuildFile)
	if p.Opt.Debug {
		cmd = fmt.Sprintf("%s build -debug %s", packerBinary, p.ProviderConfig.BuildFile)
	}
	out, _ = utils.RunCommandWithErr(cmd)

	if !strings.Contains(out, fmt.Sprintf("%v scan -f", libs.BINARY)) {
		if !strings.Contains(out, fmt.Sprintf("%v: command not found", libs.BINARY)) {
			utils.ErrorF(out)
			return fmt.Errorf("error running provisioning")
		}
	}
	return nil
}
