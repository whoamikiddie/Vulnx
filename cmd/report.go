package cmd

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/whoamikiddie/vulnx/core"
	"github.com/whoamikiddie/vulnx/utils"
)

func init() {
	var reportCmd = &cobra.Command{
		Use:   "report",
		Short: "Show report of existing workspace",
		Long:  core.Banner(),
		RunE:  runReport,
	}

	var lsCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all current existing workspace",
		Long:    core.Banner(),
		RunE:    runReportList,
	}
	reportCmd.AddCommand(lsCmd)

	var viewCmd = &cobra.Command{
		Use:     "view",
		Aliases: []string{"vi", "v"},
		Short:   "View all reports of existing workspace",
		Long:    core.Banner(),
		RunE:    runReportView,
	}
	reportCmd.AddCommand(viewCmd)

	var extractCmd = &cobra.Command{
		Use:     "extract",
		Aliases: []string{"ext", "ex", "e"},
		Short:   "Extract a compressed workspace",
		Long:    core.Banner(),
		RunE:    runReportExtract,
	}
	extractCmd.Flags().StringVar(&options.Report.ExtractFolder, "dest", "", "Destination folder to extract data to")
	reportCmd.AddCommand(extractCmd)

	var compressCmd = &cobra.Command{
		Use:     "compress",
		Aliases: []string{"com", "compr", "compres", "c"},
		Short:   "Create a backup of the selected workspace",
		Long:    core.Banner(),
		RunE:    runReportCompress,
	}
	reportCmd.AddCommand(compressCmd)

	reportCmd.PersistentFlags().BoolVar(&options.Report.Raw, "raw", false, "Show all the file in the workspace")
	reportCmd.PersistentFlags().StringVar(&options.Report.PublicIP, "ip", "", "Show downloadable file with the given IP address")
	reportCmd.PersistentFlags().BoolVar(&options.Report.Static, "static", false, "Show report file with Prefix Static")
	reportCmd.SetHelpFunc(ReportHelp)
	RootCmd.AddCommand(reportCmd)
	reportCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if options.FullHelp {
			cmd.Help()
			os.Exit(0)
		}
	}
}

func runReportList(_ *cobra.Command, _ []string) error {
	core.ListWorkspaces(options)
	return nil
}

func runReportView(_ *cobra.Command, _ []string) error {
	if options.Report.PublicIP == "" {
		if utils.GetOSEnv("IPAddress", "127.0.0.1") == "127.0.0.1" {
			options.Report.PublicIP = utils.GetOSEnv("IPAddress", "127.0.0.1")
		}
	}

	if options.Report.PublicIP == "0" || options.Report.PublicIP == "0.0.0.0" {
		options.Report.PublicIP = getPublicIP()
	}

	if len(options.Scan.Inputs) == 0 {
		core.ListWorkspaces(options)
		utils.InforF("Please select workspace to view report. Try %s", color.HiCyanString(`'osmedeus report view -t target.com'`))
		return nil
	}

	for _, target := range options.Scan.Inputs {
		core.ListSingleWorkspace(options, target)
	}
	return nil
}

func runReportExtract(_ *cobra.Command, _ []string) error {
	var err error
	if options.Report.ExtractFolder == "" {
		options.Report.ExtractFolder = options.Env.WorkspacesFolder
	} else {
		options.Report.ExtractFolder, err = filepath.Abs(filepath.Dir(options.Report.ExtractFolder))
		if err != nil {
			return err
		}
	}

	for _, input := range options.Scan.Inputs {
		core.ExtractBackup(input, options)

		target := strings.ReplaceAll(path.Base(input), ".tar.gz", "")
		core.ListSingleWorkspace(options, target)
	}

	return nil
}

func runReportCompress(_ *cobra.Command, _ []string) error {
	for _, target := range options.Scan.Inputs {
		core.CompressWorkspace(target, options)
	}
	return nil
}

func runReport(_ *cobra.Command, args []string) error {
	if options.Report.PublicIP == "" {
		if utils.GetOSEnv("IPAddress", "127.0.0.1") == "127.0.0.1" {
			options.Report.PublicIP = utils.GetOSEnv("IPAddress", "127.0.0.1")
		}
	}

	if options.Report.PublicIP == "0" || options.Report.PublicIP == "0.0.0.0" {
		options.Report.PublicIP = getPublicIP()
	}

	if len(args) == 0 {
		core.ListWorkspaces(options)
	}

	return nil
}

func getPublicIP() string {
	utils.DebugF("getting Public IP Address")
	req, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "127.0.0.1"
	}
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "127.0.0.1"
	}
	return string(body)
}
