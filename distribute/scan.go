package distribute

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/whoamikiddie/vulnx/core"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/provider"
	"github.com/whoamikiddie/vulnx/utils"
)

func (c *CloudRunner) PrepareInput() {
	c.Opt.Scan.ROptions = c.Target
	c.Opt.Scan.Flow = "cloud-distributed"
	//database.NewScan(c.Opt, "cli")
	c.Input = c.Target["Target"]
	runner, err := core.InitRunner(c.Input, c.Opt)
	if err == nil {
		c.Runner = runner
		c.Runner.RunnerType = "cloud"
	}

	// for creating local DB record
	if c.Opt.Cloud.Flow != "" {
		c.TaskName = c.Opt.Cloud.Flow
		c.TaskType = "flow"
	} else {
		c.TaskName = c.Opt.Cloud.Flow
		c.TaskType = "module"
	}
	c.CloudMoreParams()
	// more params from -p flag
	if len(c.Opt.Cloud.Params) > 0 {
		params := core.ParseParams(c.Opt.Cloud.Params)
		if len(params) > 0 {
			for k, v := range params {
				v = core.ResolveData(v, c.Target)
				c.Target[k] = v
			}
		}
	}

}

func (c *CloudRunner) StartScan() error {
	c.DBNewTarget()
	// c.DBNewScanLocal()
	// c.DBNewCloudInstance()

	err := c.RunScan()
	if err != nil {
		return fmt.Errorf("error to start the scan")
	}

	// utils.DebugF("Create UI report for %s: %s", c.DestInstance, color.HiCyanString(c.Opt.Cloud.RawCommand))
	// c.Runner.DBDoneScan()
	return nil
}

func (c *CloudRunner) RunScan() error {
	// -f mean run in a background
	// ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i <private_key> root@IP  -f <command>
	// -t mean run and wait
	// ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i <private_key> root@IP  -t <command>
	if c.Opt.Cloud.RawCommand != "" {
		c.Opt.Cloud.RawCommand = core.ResolveData(c.Opt.Cloud.RawCommand, c.Target)
	} else {
		c.Opt.Cloud.RawCommand = CommandBuilder(c.Opt)
	}
	c.Opt.Cloud.RawCommand = core.ResolveData(c.Opt.Cloud.RawCommand, c.Target)
	c.RawCommand = c.Opt.Cloud.RawCommand

	// init tmux session
	out, err := c.SSHExec("tmux new-session -d -t main")

	// boot the instance again if it still didn't up
	if err != nil && strings.Contains(err.Error(), "time out") {
		time.Sleep(60 * time.Second)
		c.Provider.Action(provider.BootInstance, c.InstanceID)
		out, err = c.SSHExec("tmux new-session -d -t main")
	}

	// really run main command
	tcmd := fmt.Sprintf(`"%s"`, c.Opt.Cloud.RawCommand)
	_, err = c.SSHExec(fmt.Sprintf(`tmux send-keys %s ENTER`, tcmd))

	// still error then it must be something wrong
	if err != nil {
		utils.ErrorF("An error occurred with %v", color.HiYellowString(c.DestInstance))
		utils.ErrorF("error log: %v", out)
		return fmt.Errorf("error running command on %v", color.HiYellowString(c.DestInstance))
	}
	utils.InforF("Start to run the scan %v with command %v", color.HiYellowString(c.DestInstance), color.HiCyanString(c.Opt.Cloud.RawCommand))

	// wait a bit for process really start
	time.Sleep(60 * time.Second)
	if !c.IsRunning() {
		return fmt.Errorf("Failed to initiate the scan on %v", color.HiYellowString(c.DestInstance))
	}

	c.WriteInstanceConfig()
	return nil
}

func (c *CloudRunner) CheckingDone() error {
	if c.Opt.Cloud.NoDelete {
		time.Sleep(60 * time.Second)
		return nil
	}
	utils.InforF("Checking scan process at: %s", color.HiBlueString(c.PublicIP))

	// dest := fmt.Sprintf("%s/.%s/workspaces/%s/done", c.BasePath, libs.BINARY, c.Target["Workspace"])
	// @NOTE: this is new workspaces folder
	dest := fmt.Sprintf("%s/workspaces-%s/%s/done", c.BasePath, libs.BINARY, c.Target["Workspace"])

	cmd := fmt.Sprintf("file %s", dest)
	out, _ := c.SSHExec(cmd)

	if strings.Contains(out, "ASCII text") || strings.Contains(out, "JSON data") {
		return nil
	}

	waitTime := utils.CalcTimeout(c.Opt.Cloud.ClearTime)
	counter := 1
	for {
		time.Sleep(time.Duration(waitTime) * time.Second)
		out, _ = c.SSHExec(cmd)
		if strings.Contains(out, "ASCII text") || strings.Contains(out, "JSON data") {
			utils.InforF("The scan is done at: %s", color.HiBlueString(c.PublicIP))
			return nil
		}

		if !c.IsRunning() {
			return fmt.Errorf("no process running at %v", c.PublicIP)
		}

		// check if we have panic or not
		if c.IsPanic() {
			return fmt.Errorf("panic detected at %v", c.PublicIP)
		}

		if counter%50 == 0 {
			c.SyncResult()
		}
		counter++
	}
}

// below code is  experimental part

func (c *CloudRunner) SyncResult() error {
	target := c.Opt.Cloud.Input
	if !c.Provider.IsBackgroundCheck {
		utils.InforF("Sync back the data of taget %v from %v", color.HiCyanString(target), color.HiYellowString(c.DestInstance))
	}

	if c.Opt.Cloud.LocalSyncFolder == "" {
		c.Opt.Cloud.LocalSyncFolder = fmt.Sprintf("%s/workspaces-%s/", c.BasePath, libs.BINARY)
	}

	// on vps machine
	src := c.Opt.Cloud.LocalSyncFolder

	// on local
	dest := path.Join(c.Opt.Env.WorkspacesFolder, c.Opt.Cloud.BaseWorkspace)
	utils.MakeDir(dest)

	cmd := fmt.Sprintf("rsync -e 'ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s' -avzr --progress %s:%s %s", c.SshPrivateKey, c.DestInstance, src, dest)

	c.RetryCommandWithExpectString(cmd, `bytes/sec`)
	if !utils.FolderExists(dest) {
		utils.ErrorF("error sync result back from: %v to %v", c.DestInstance, dest)
	}

	return nil
}

func (c *CloudRunner) CopyTarget() error {
	target := c.Opt.Cloud.Input
	utils.DebugF("Sync input of %s to %s", target, c.DestInstance)

	dest := c.Target["Target"]
	if !utils.FileExists(dest) && !utils.FolderExists(dest) {
		utils.DebugF("target is not a file: %s", dest)
		return nil
	}

	if c.Opt.Cloud.EnableChunk {
		dest = c.Opt.Cloud.ChunkInputs
	}

	c.SSHExec(fmt.Sprintf("mkdir -p %s", path.Dir(dest)))
	cmd := fmt.Sprintf("rsync -e 'ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s' -avzr --progress %s %s:%s", c.SshPrivateKey, dest, c.DestInstance, dest)
	c.RetryCommandWithExpectString(cmd, `bytes/sec`)
	return nil
}

func (c *CloudRunner) CopyWorkflow() error {
	utils.DebugF("Sync workflow of %s to %s", c.Opt.Env.WorkFlowsFolder, c.DestInstance)
	destWorkflow := fmt.Sprintf("%v/osmedeus-base/", c.BasePath)
	if c.Opt.Cloud.RemoteWorkflowFolder != "" {
		destWorkflow = c.Opt.Cloud.RemoteWorkflowFolder
	}

	// c.SSHExec(fmt.Sprintf("rm -rf %s && mkdir -p %s", destWorkflow, destWorkflow))
	cmd := fmt.Sprintf("rsync -e 'ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s' -avzr --progress %s %s:%s", c.SshPrivateKey, c.Opt.Env.WorkFlowsFolder, c.DestInstance, destWorkflow)
	c.RetryCommandWithExpectString(cmd, `bytes/sec`)
	return nil
}

func (c *CloudRunner) PreRunRemote() error {
	if len(c.Opt.Cloud.RemotePreRun) <= 0 {
		return nil
	}
	utils.InforF("Run remote command on: %s", c.PublicIP)

	// really start to run pre commands
	for _, rcmd := range c.Opt.Cloud.RemotePreRun {
		cmd := core.ResolveData(rcmd, c.Target)
		utils.InforF("Run pre command on %s: %s", c.PublicIP, cmd)
		c.SSHExec(cmd)
	}
	return nil
}

func (c *CloudRunner) PreRunLocal() error {
	if len(c.Opt.Cloud.LocalPreRun) <= 0 {
		return nil
	}
	c.Opt.Scan.ROptions = c.Target
	utils.InforF("Start %v", color.HiCyanString("PreRunLocal"))

	// really start to run pre commands
	for _, script := range c.Opt.Cloud.LocalPreRun {
		script = core.ResolveData(script, c.Target)
		c.Runner.RunScript(script)
	}
	return nil
}

func (c *CloudRunner) PostRunLocal() error {
	c.Opt.Scan.ROptions = c.Target

	if len(c.Opt.Cloud.LocalSteps) > 0 {
		// for running local steps
		utils.DebugF("Running local steps")
		for _, step := range c.Opt.Cloud.LocalSteps {
			c.Runner.RunStep(step)
		}
	}

	if len(c.Opt.Cloud.LocalPostRun) <= 0 {
		return nil
	}

	utils.InforF("Start %v", color.HiCyanString("PostRunLocal"))
	// really start to run pre commands
	for _, script := range c.Opt.Cloud.LocalPostRun {
		script = core.ResolveData(script, c.Target)
		c.Runner.RunScript(script)
	}
	return nil
}
