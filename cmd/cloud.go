package cmd

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/whoamikiddie/vulnx/core"
	"github.com/whoamikiddie/vulnx/distribute"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	var cloudCmd = &cobra.Command{
		Use:   "cloud",
		Short: "Perform a scan using the distributed cloud mode",
		Long:  core.Banner(),
		RunE:  runCloud,
	}

	// core options
	cloudCmd.Flags().StringVarP(&options.Cloud.Module, "module", "m", "", "module name for running")
	cloudCmd.Flags().StringVarP(&options.Cloud.Flow, "flow", "f", "general", "Flow name for running (default: general)")
	cloudCmd.Flags().StringVarP(&options.Cloud.Workspace, "workspace", "w", "", "Name of workspace (default is same as target)")
	cloudCmd.Flags().StringSliceVarP(&options.Cloud.Params, "params", "p", []string{}, "Custom params -p='foo=bar' (Multiple -p flags are accepted)")

	// chunk inputs
	cloudCmd.Flags().BoolVar(&options.Cloud.EnablePrivateIP, "privateIP", false, "Enable Private IP")
	cloudCmd.Flags().BoolVar(&options.Cloud.TargetAsFile, "as-file", false, "Run target as file (use -T targets.txt file instead of -t targets.txt at cloud instance)")
	cloudCmd.Flags().StringVar(&options.Cloud.LocalSyncFolder, "rfolder", "", "Remote Folder to sync back to local")

	// commands on cloud
	cloudCmd.Flags().IntVar(&options.Cloud.Threads, "cloud-threads", 1, "Concurrency level on remote cloud")
	cloudCmd.Flags().StringVar(&options.Cloud.Extra, "extra", "", "append raw command after the command builder")
	cloudCmd.Flags().StringVar(&options.Cloud.RawCommand, "cmd", "", "specific raw command and override everything (eg: --cmd 'curl {{Target}}')")
	cloudCmd.Flags().StringVar(&options.Cloud.ClearTime, "clear", "10m", "time to wait before next clear check")
	cloudCmd.Flags().StringVar(&options.Cloud.TempTarget, "tempTargets", "/tmp/osm-tmp-inputs/", "Temp Folder to store targets file")

	// mics option
	cloudCmd.Flags().BoolVar(&options.Cloud.EnableSyncWorkflow, "sync-workflow", false, "Enable Sync Workflow folder to remote machine first before starting the scan")
	cloudCmd.Flags().BoolVarP(&options.Cloud.CopyWorkspaceToGit, "gws", "G", false, "Enable Copy Workspace to Git (run -f sync after done)")
	cloudCmd.Flags().BoolVarP(&options.Cloud.DisableLocalSync, "no-lsync", "z", false, "Disable sync back data to local machine")
	cloudCmd.Flags().BoolVar(&options.Cloud.BackgroundRun, "bg", false, "Send command to instance without checking if process is done or not")
	cloudCmd.Flags().BoolVar(&options.Cloud.EnableTerraform, "tf", false, "Use terraform to create cloud instance")
	cloudCmd.Flags().BoolVar(&options.Cloud.NoDelete, "no-del", false, "Don't delete instance after done (you can run 'osmedeus provider health' to clean up later)")
	cloudCmd.Flags().BoolVar(&options.Cloud.IgnoreProcess, "no-ps", false, "Disable checking process on remote machine")
	cloudCmd.Flags().IntVar(&options.Cloud.Retry, "retry", 10, "Number of retry when command is error")
	cloudCmd.SetHelpFunc(CloudHelp)
	RootCmd.AddCommand(cloudCmd)
	cloudCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if options.FullHelp {
			cmd.Help()
			os.Exit(0)
		}
	}
}

func runCloud(cmd *cobra.Command, _ []string) error {
	utils.GoodF("Using the %v Engine %v by %v", cases.Title(language.Und, cases.NoLower).String(libs.BINARY), color.HiCyanString(libs.VERSION), color.HiMagentaString(libs.AUTHOR))
	utils.InforF("Storing the log file to: %v", color.CyanString(options.LogFile))

	// parse some arguments
	threads, _ := cmd.Flags().GetInt("thread")
	if threads > 1 || options.Cloud.Threads <= 1 {
		options.Cloud.Threads = threads
	}

	// get pre run commands
	getPreRun(&options)

	// change targets list if chunk mode enable
	if options.Cloud.EnableChunk {
		utils.InforF("Running cloud scan in chunk mode")
		for _, target := range options.Scan.Inputs {
			chunkTargets := HandleChunksInputs(target)
			if len(chunkTargets) == 0 {
				continue
			}

			distribute.InitCloud(options, chunkTargets)
			// remove chunk inputs
			utils.DebugF("Remove chunk inputs file")
			for _, ctarget := range chunkTargets {
				os.RemoveAll(ctarget)
			}
		}
		return nil
	}

	// @NOTE: pro-tips
	if options.Concurrency > 1 && len(options.Scan.Inputs) == 1 {
		if !utils.FileExists(options.Scan.Inputs[0]) {
			utils.WarnF("You're using %v in cloud scan but your input %v is just a single domain", color.HiMagentaString(`'-c %v'`, options.Concurrency), color.HiMagentaString(`'-t %v'`, options.Scan.Inputs[0]))
			utils.WarnF("Consider running: osmedeus cloud -c 5 -T list-of-targets.txt")
		}
	}

	distribute.InitCloud(options, options.Scan.Inputs)
	return nil
}

// HandleChunksInputs split the inputs to multiple file first
func HandleChunksInputs(target string) []string {
	var chunkTargets []string
	utils.MakeDir(options.Cloud.ChunkInputs)

	if !utils.FileExists(target) {
		utils.ErrorF("error to split input file: %v", target)
		return chunkTargets
	}

	if options.Cloud.NumberOfParts == 0 {
		options.Cloud.NumberOfParts = options.Concurrency
	}

	utils.DebugF("Splitting %v to %v part", target, options.Cloud.NumberOfParts)
	rawChunks, err := utils.SplitLineChunks(target, options.Cloud.NumberOfParts)
	if err != nil || len(rawChunks) == 0 {
		utils.ErrorF("error to split input file: %v", target)
		return chunkTargets
	}
	fp, err := os.Open(target)
	if err != nil {
		utils.ErrorF("error to open input file: %v", target)
		return chunkTargets
	}
	for index, offset := range rawChunks {
		targetName := fmt.Sprintf("%s-chunk-%v", utils.CleanPath(target), index)
		targetName = path.Join(options.Cloud.ChunkInputs, targetName)

		sectionReader := io.NewSectionReader(fp, offset.Start, offset.Stop-offset.Start)
		targetFile, err := os.OpenFile(targetName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			utils.ErrorF("error when create chunk file: %v", target)
			continue
		}

		_, err = io.Copy(targetFile, sectionReader)
		if err != nil {
			utils.ErrorF("error to read chunk file: %s", err)
			continue
		}
		targetFile.Close()
		chunkTargets = append(chunkTargets, targetName)
	}

	return chunkTargets
}

func getPreRun(options *libs.Options) {
	if options.Cloud.Module != "" {
		module := core.DirectSelectModule(*options, options.Cloud.Module)
		if module == "" {
			utils.ErrorF("Error to select module: %s", options.Cloud.Module)
			return
		}
		parsedModule, err := core.ParseModules(module)
		if err == nil {
			options.Cloud.RemotePreRun = parsedModule.RemotePreRun
			options.Cloud.LocalPostRun = parsedModule.LocalPostRun
			options.Cloud.LocalPreRun = parsedModule.LocalPreRun
			options.Cloud.LocalSteps = parsedModule.LocalSteps
		}
		return
	}

	if options.Cloud.Flow != "" {
		flows := core.SelectFlow(options.Cloud.Flow, *options)
		for _, flow := range flows {
			parseFlow, err := core.ParseFlow(flow)
			if err == nil {
				options.Cloud.RemotePreRun = parseFlow.RemotePreRun
				options.Cloud.LocalPostRun = parseFlow.LocalPostRun
				options.Cloud.LocalPreRun = parseFlow.LocalPreRun
			}
		}
	}
}
