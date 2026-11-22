package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

const gitPipelinesListUsage = "penggunaan: shikigamictl git pipelines --workspace <id> --project <id>"
const gitPipelineGetUsage = "penggunaan: shikigamictl git pipeline --workspace <id> --project <id> --id <id>"

type gitPipelinesItem struct {
	ID          string `json:"id"`
	Ref         string `json:"ref"`
	SHA         string `json:"sha"`
	Status      string `json:"status"`
	Author      string `json:"author"`
	DurationSec int    `json:"durationSec"`
	UpdatedAt   string `json:"updatedAt"`
	URL         string `json:"url"`
}

type gitPipelinesJob struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	DurationSec int    `json:"durationSec"`
	URL         string `json:"url"`
}

type gitPipelinesStage struct {
	Name string            `json:"name"`
	Jobs []gitPipelinesJob `json:"jobs"`
}

type gitPipelinesDetail struct {
	gitPipelinesItem
	Stages []gitPipelinesStage `json:"stages"`
}

var gitPipelinesOneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func gitPipelinesDuration(sec int) string {
	if sec <= 0 {
		return "-"
	}
	return fmt.Sprintf("%ds", sec)
}

func gitPipelinesShortSHA(sha string) string {
	s := gitPipelinesOneLine.Replace(sha)
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func gitPipelinesLine(it gitPipelinesItem) string {
	return strings.Join([]string{
		gitPipelinesOneLine.Replace(it.ID),
		gitPipelinesOneLine.Replace(it.Ref),
		gitPipelinesOneLine.Replace(it.Status),
		gitPipelinesShortSHA(it.SHA),
		gitPipelinesDuration(it.DurationSec),
		gitPipelinesOneLine.Replace(it.URL),
	}, "\t")
}

func gitPipelinesParse(name, usage string, withID bool, args []string) (ws, project, id string, err error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	wsFlag := fs.String("workspace", "", "")
	projectFlag := fs.String("project", "", "")
	idFlag := new(string)
	if withID {
		idFlag = fs.String("id", "", "")
	}
	if perr := fs.Parse(args); perr != nil {
		return "", "", "", fmt.Errorf("%v\n%s", perr, usage)
	}
	if fs.NArg() > 0 {
		return "", "", "", errors.New(usage)
	}
	ws = strings.TrimSpace(*wsFlag)
	project = strings.TrimSpace(*projectFlag)
	id = strings.TrimSpace(*idFlag)
	if ws == "" || project == "" || (withID && id == "") {
		return "", "", "", errors.New(usage)
	}
	return ws, project, id, nil
}

func gitPipelines(_ context.Context, c *ipc.Client, args []string) error {
	ws, project, _, err := gitPipelinesParse("git pipelines", gitPipelinesListUsage, false, args)
	if err != nil {
		return err
	}
	var res []gitPipelinesItem
	if err := c.Call("git.pipelines.list", map[string]any{
		"workspaceId": ws, "projectId": project,
	}, &res); err != nil {
		return err
	}
	for _, it := range res {
		fmt.Println(gitPipelinesLine(it))
	}
	fmt.Printf("%d baris\n", len(res))
	return nil
}

func gitPipeline(_ context.Context, c *ipc.Client, args []string) error {
	ws, project, id, err := gitPipelinesParse("git pipeline", gitPipelineGetUsage, true, args)
	if err != nil {
		return err
	}
	var res gitPipelinesDetail
	if err := c.Call("git.pipeline.get", map[string]any{
		"workspaceId": ws, "projectId": project, "id": id,
	}, &res); err != nil {
		return err
	}
	fmt.Println(gitPipelinesLine(res.gitPipelinesItem))
	for _, st := range res.Stages {
		fmt.Printf("[stage] %s\n", gitPipelinesOneLine.Replace(st.Name))
		for _, j := range st.Jobs {
			fmt.Printf("  %s\t%s\t%s\t%s\n",
				gitPipelinesOneLine.Replace(j.Name),
				gitPipelinesOneLine.Replace(j.Status),
				gitPipelinesDuration(j.DurationSec),
				gitPipelinesOneLine.Replace(j.URL))
		}
	}
	return nil
}

func init() {
	registerGitSub("pipelines", gitPipelines)
	registerGitSub("pipeline", gitPipeline)
}
