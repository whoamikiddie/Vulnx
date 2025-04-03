package server

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/whoamikiddie/vulnx/core"
	"github.com/whoamikiddie/vulnx/execution"
	"github.com/whoamikiddie/vulnx/libs"
)

func Process(c *fiber.Ctx) error {
	processes := execution.GetOsmProcess("")
	return c.JSON(ResponseHTTP{
		Status:  200,
		Data:    processes,
		Type:    "processes",
		Total:   len(processes),
		Message: "List all osm process",
	})
}

func RawWorkspace(c *fiber.Ctx) error {
	return c.JSON(ResponseHTTP{
		Status: 200,
		Data: fiber.Map{
			"storages":   fmt.Sprintf("/%s/storages/", Opt.Server.StaticPrefix),
			"workspaces": fmt.Sprintf("/%s/workspaces/", Opt.Server.StaticPrefix),
			"logs":       fmt.Sprintf("/%s/logs/", Opt.Server.StaticPrefix),
		},
		Type:    "raw",
		Message: "Raw directory",
	})
}

func ListFlows(c *fiber.Ctx) error {
	flows := core.ListFlow(Opt)
	if len(flows) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Can't list workflow",
		})
	}

	var result []map[string]string

	for _, flow := range flows {
		if flow != "" {
			item := make(map[string]string)
			item["name"] = strings.TrimSuffix(filepath.Base(flow), ".yaml")

			// get modules
			Opt.Flow.Type = strings.TrimSuffix(item["name"], path.Ext(item["name"]))
			rawModules := core.ListModules(Opt)
			var modules []string
			for _, module := range rawModules {
				if module != "" {
					modules = append(modules, strings.TrimSuffix(filepath.Base(module), ".yaml"))
				}
			}

			item["desc"] = ""
			parsedFlow, err := core.ParseFlow(flow)
			if err == nil {
				item["desc"] = parsedFlow.Desc
			}

			item["modules"] = strings.Join(modules, ",")
			result = append(result, item)

		}
	}

	return c.JSON(ResponseHTTP{
		Status:  200,
		Data:    result,
		Total:   len(flows),
		Type:    "flows",
		Message: "Workflows Listing",
	})
}

func HelperMessage(c *fiber.Ctx) error {
	message := fmt.Sprintf(`
[*] Visit this page for complete Usage: %s
`, libs.DOCS)

	return c.JSON(ResponseHTTP{
		Status: 200,
		Data: fiber.Map{
			"version": libs.VERSION,
			"doc":     libs.DOCS,
			"message": message,
		},
		Type:    "helper",
		Message: "Helper message",
	})
}
