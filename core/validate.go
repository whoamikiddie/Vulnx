package core

import (
	"fmt"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/go-playground/validator/v10"
	"github.com/whoamikiddie/vulnx/libs"
	"github.com/whoamikiddie/vulnx/utils"
)

func (r *Runner) Validator() error {
	if r.RequiredInput == "" || r.Opt.DisableValidateInput {
		return nil
	}

	r.RequiredInput = strings.ToLower(strings.TrimSpace(r.RequiredInput))
	var inputAsFile bool
	// cidr, cidr-file
	if strings.HasSuffix(r.RequiredInput, "-file") || r.RequiredInput == "file" {
		inputAsFile = true
	}
	v := validator.New()

	// if input as a file
	if utils.FileExists(r.Input) && inputAsFile {
		r.InputType = "file"
		inputs := utils.ReadingLines(r.Input)

		for index, input := range inputs {
			if strings.TrimSpace(input) == "" {
				continue
			}
			// no more validation if it's file 'validator: file'
			if r.RequiredInput == "file" {
				continue
			}

			inputType, err := validate(v, input)
			// fmt.Println("r.RequiredInput, inputType", r.RequiredInput, inputType)
			if err == nil {
				// really validate thing
				if !strings.HasPrefix(r.RequiredInput, inputType) {
					utils.DebugF("validate: %v -- %v", input, inputType)
					errString := fmt.Sprintf("line %v in %v file not match the require input: %v -- %v", index, r.Input, input, inputType)
					utils.ErrorF(errString)
					return fmt.Errorf(errString)
				}
			}
		}
		return nil

	}

	var err error
	r.InputType, err = validate(v, r.Input)
	if err != nil {
		utils.ErrorF("unrecognized input: %v", r.Input)
		return err
	}
	utils.InforF("Start validating input: %v -- %v", color.HiCyanString(r.Input), color.HiCyanString(r.InputType))

	if !strings.HasPrefix(r.RequiredInput, r.InputType) {
		return fmt.Errorf("input does not match the require validation: inputType:%v -- requireType:%v", r.InputType, r.RequiredInput)
	}

	if inputAsFile {
		utils.MakeDir(libs.TEMP)
		suffix := utils.RandomString(4)
		if r.Opt.Scan.SuffixName != "" {
			suffix = r.Opt.Scan.SuffixName + "-" + utils.RandomString(4)
		}
		dest := path.Join(libs.TEMP, fmt.Sprintf("%v-%v", utils.StripPath(r.Input), suffix))
		if r.Opt.Scan.CustomWorkspace != "" {
			dest = path.Join(libs.TEMP, fmt.Sprintf("%v-%v", utils.StripPath(r.Opt.Scan.CustomWorkspace), suffix))
		}
		utils.WriteToFile(dest, r.Input)
		utils.InforF("Convert input to a file: %v", dest)
		r.Input = dest
		r.Target = ParseInput(r.Input, r.Opt)
	}

	utils.DebugF("validator: input:%v -- type: %v -- require:%v", r.Input, r.InputType, r.RequiredInput)
	return nil
}

func validate(v *validator.Validate, raw string) (string, error) {
	var err error
	var inputType string

	if utils.FileExists(raw) {
		inputType = "file"
	}

	err = v.Var(raw, "required,url")
	if err == nil {
		inputType = "url"
	}

	err = v.Var(raw, "required,ipv4")
	if err == nil {
		inputType = "ip"
	}

	err = v.Var(raw, "required,fqdn")
	if err == nil {
		inputType = "domain"
	}

	err = v.Var(raw, "required,hostname")
	if err == nil {
		inputType = "domain"
	}

	err = v.Var(raw, "required,cidr")
	if err == nil {
		inputType = "cidr"
	}

	err = v.Var(raw, "required,uri")
	if err == nil {
		inputType = "url"
	}

	err = v.Var(raw, "required,uri")
	if err == nil {
		inputType = "url"
		if strings.HasPrefix(raw, "https://github.com") || strings.HasPrefix(raw, "https://gitlab.com") {
			inputType = "git-url"
		}
	}

	if strings.HasPrefix(raw, "git@") {
		inputType = "git-url"
	}

	if inputType == "" {
		return "", fmt.Errorf("unrecognized input")
	}

	return inputType, nil
}
