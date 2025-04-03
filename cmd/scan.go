package cmd

import (
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/panjf2000/ants"
	"github.com/spf13/cobra"
	"github.com/whoamikiddie/vulnx/core"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	var scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Conduct a scan following a predetermined flow/module",
		Long:  core.Banner(),
		RunE:  runScan,
	}

	scanCmd.SetHelpFunc(ScanHelp)
	RootCmd.AddCommand(scanCmd)
	scanCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if options.FullHelp {
			cmd.Help()
			os.Exit(0)
		}
	}
}

func runScan(_ *cobra.Command, _ []string) error {
	utils.GoodF("Using the %v Engine %v by %v", cases.Title(language.Und, cases.NoLower).String(libs.BINARY), color.HiCyanString(libs.VERSION), color.HiMagentaString(libs.AUTHOR))
	utils.InforF("Storing the log file to: %v", color.CyanString(options.LogFile))

	var wg sync.WaitGroup
	p, _ := ants.NewPoolWithFunc(options.Concurrency, func(i interface{}) {
		// really start to scan
		CreateRunner(i)
		wg.Done()
	}, ants.WithPreAlloc(true))
	defer p.Release()

	if options.Cloud.EnableChunk {
		for _, target := range options.Scan.Inputs {
			chunkTargets := HandleChunksInputs(target)
			for _, chunkTarget := range chunkTargets {
				wg.Add(1)
				_ = p.Invoke(chunkTarget)
			}
		}
	} else {
		for _, target := range options.Scan.Inputs {
			wg.Add(1)
			_ = p.Invoke(strings.TrimSpace(target))
		}
	}

	wg.Wait()
	return nil
}

func CreateRunner(j interface{}) {
	target := j.(string)
	if core.IsRootDomain(target) && options.Scan.Flow == "general" && len(options.Scan.Modules) == 0 {
		utils.WarnF("looks like you scanning a subdomain '%s' with general flow. The result might be much less than usual", color.HiCyanString(target))
		utils.WarnF("Better input should be root domain with TLD like '-t target.com'")
	}

	runner, err := core.InitRunner(target, options)
	if err != nil {
		utils.ErrorF("Error init runner with: %s", target)
		return
	}
	runner.Start()
}
