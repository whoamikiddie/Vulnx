package core

import (
	"github.com/fatih/color"
	"github.com/jasonlvhit/gocron"
	"github.com/spf13/cast"
	"github.com/whoamikiddie/vulnx/utils"
)

func taskWithParams(cmd string) {
	utils.InforF("Exec: %v", color.HiMagentaString(cmd))
	_, err := utils.RunCommandSteamOutput(cmd)
	if err != nil {
		utils.ErrorF("Error running command: %v", err)
	}
}

func RunCron(cmd string, schedule int) {

	if schedule == -1 {
		utils.InforF("Run command forever: %v", cmd)
		for {
			taskWithParams(cmd)
		}
	}

	utils.InforF("Start cron job with %v seconds: %v", schedule, color.HiCyanString(cmd))
	gocron.Every(cast.ToUint64(schedule)).Minutes().Do(taskWithParams, cmd)
	<-gocron.Start()
}
