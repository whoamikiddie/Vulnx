package database

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
)

func GetAllWorkspaces(opt libs.Options) (directories []string) {
	// Open the specified directory
	dir, err := os.Open(opt.Env.WorkspacesFolder)
	if err != nil {
		return directories
	}
	defer dir.Close()

	// Read all entries in the directory
	entries, err := dir.Readdir(-1)
	if err != nil {
		return directories
	}

	for _, entry := range entries {
		wsName := entry.Name()
		directories = append(directories, wsName)
	}

	return directories
}

func GetAllScan(opt libs.Options) (scans []Scan) {
	wss := GetAllWorkspaces(opt)
	for _, wsName := range wss {
		runtimeFile := filepath.Join(opt.Env.WorkspacesFolder, wsName, "runtime")
		if !utils.FileExists(runtimeFile) {
			continue
		}

		// parse the content
		runtimeContent := utils.GetFileContent(runtimeFile)
		wsData := Scan{}
		if err := jsoniter.UnmarshalFromString(runtimeContent, &wsData); err == nil {

			// replace the filepath with static prefix
			wsData.MarkDownReport = strings.ReplaceAll(wsData.MarkDownReport, opt.Env.WorkspacesFolder, path.Join("/", opt.Server.StaticPrefix, "workspaces"))
			wsData.MarkDownSunmmary = strings.ReplaceAll(wsData.MarkDownSunmmary, opt.Env.WorkspacesFolder, path.Join("/", opt.Server.StaticPrefix, "workspaces"))
			scans = append(scans, wsData)
		}

	}
	return scans
}

func GetSingleScan(wsName string, opt libs.Options) (scan Scan) {
	runtimeFile := filepath.Join(opt.Env.WorkspacesFolder, wsName, "runtime")
	if !utils.FileExists(runtimeFile) {
		return scan
	}

	// parse the content
	runtimeContent := utils.GetFileContent(runtimeFile)
	if err := jsoniter.UnmarshalFromString(runtimeContent, &scan); err == nil {
		// replace the filepath with static prefix
		scan.MarkDownReport = strings.ReplaceAll(scan.MarkDownReport, opt.Env.WorkspacesFolder, path.Join("/", opt.Server.StaticPrefix, "workspaces"))
		scan.MarkDownSunmmary = strings.ReplaceAll(scan.MarkDownSunmmary, opt.Env.WorkspacesFolder, path.Join("/", opt.Server.StaticPrefix, "workspaces"))
		return scan
	}
	return scan
}

func GetScanProgress(opt libs.Options) (scans []Scan) {
	rawScans := GetAllScan(opt)

	for _, scan := range rawScans {
		scan.Target = Target{}
		scans = append(scans, scan)
	}

	return scans
}

// func GetWorkspaceDetail(wsName string, opt libs.Options) (workspace Scan) {
// 	runtimeFile := filepath.Join(opt.Env.WorkspacesFolder, wsName, "runtime")
// 	if !utils.FileExists(runtimeFile) {
// 		return workspace
// 	}

// 	// parse the content
// 	runtimeContent := utils.GetFileContent(runtimeFile)
// 	if err := jsoniter.UnmarshalFromString(runtimeContent, &workspace); err == nil {
// 		return workspace
// 	}

// 	return workspace
// }
