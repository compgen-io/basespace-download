package main

import (
	"encoding/json"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/mitchellh/ioprogress"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var basespaceApiUrl = "https://api.basespace.illumina.com/v1pre3"

func main() {
	app := cli.NewApp()
	app.Name = "basespace-download"
	app.Version = "basespace-download"
	app.Usage = "basespace-download - Basespace file downloader"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "t", Value: "", Usage: "Application token from Basespace"},
		cli.StringFlag{Name: "s", Value: "", Usage: "Sample ID to download"},
		cli.StringFlag{Name: "p", Value: "", Usage: "Project ID to download (all samples)"},
		cli.BoolFlag{Name: "dr", Usage: "Dry-run (don't download files)"},
	}

	app.Action = func(c *cli.Context) {
		if c.String("t") == "" {
			fmt.Fprintf(os.Stderr, "Missing app-token! You must obtain an Application Token from Illumina!\n\n")
			os.Exit(1)
		} else if c.String("s") != "" {
			downloadSample(c.String("t"), c.String("s"), "", "", c.Bool("dr"))
		} else if c.String("p") != "" {
			downloadProject(c.String("t"), c.String("p"), c.Bool("dr"))
		} else {
			fmt.Fprintf(os.Stderr, "You must specify either a sample (-s) or project (-p) to download!\n\n")
			os.Exit(1)
		}
	}

	app.Run(os.Args)

}

func downloadSample(token, sampleId, sampleName, prefix string, dryrun bool) {
	if sampleName == "" {
		sampleName = getSampleName(token, sampleId)
	}
	fmt.Fprintf(os.Stderr, "%sSample: [%s] %s\n", prefix, sampleId, sampleName)

	offset := 0
	total := 0

	for total == 0 || offset < total {
		url := fmt.Sprintf("%s/samples/%s/files?Offset=%d&access_token=", basespaceApiUrl, sampleId, offset)

		resp, err := http.Get(url + token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading URL: %s\n\n", url)
			os.Exit(1)
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing reading result: %s\n\n", string(body))
			os.Exit(1)
		}

		var f map[string]interface{}

		if err = json.Unmarshal(body, &f); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing JSON: %s\n\n", string(body))
			os.Exit(1)
		}

		for k, v := range f {
			if k == "Response" {
				for k2, v2 := range v.(map[string]interface{}) {
					if k2 == "Items" {
						items := v2.([]interface{})
						for i := range items {
							v3 := items[i].(map[string]interface{})
							fileId := v3["Id"].(string)
							filename := v3["Name"].(string)
							fileSize := v3["Size"].(float64)
							downloadFile(token, fileId, filename, int64(fileSize), prefix+"  ", dryrun)
						}
					}
				}
				total = int(v.(map[string]interface{})["TotalCount"].(float64))
				displayed := int(v.(map[string]interface{})["DisplayedCount"].(float64))
				offset += displayed
			}
		}
	}

}

func downloadFile(token, fileId, filename string, fileSize int64, prefix string, dryrun bool) {
	fmt.Fprintf(os.Stderr, "%s%s\n", prefix, filename)

	if dryrun {
		return
	}

	url := basespaceApiUrl + "/files/" + fileId + "/content?access_token="
	resp, err := http.Get(url + token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading URL: %s\n\n", url)
		os.Exit(1)
	}

	out, err := os.Create(filename)
	defer out.Close()

	defer resp.Body.Close()

	bar := ioprogress.DrawTextFormatBar(20)
	fmtfunc := func(progress, total int64) string {
		return fmt.Sprintf(
			"%s%s %s",
			prefix,
			bar(progress, total),
			ioprogress.DrawTextFormatBytes(progress, total))
	}

	progressR := &ioprogress.Reader{
		Reader:   resp.Body,
		Size:     fileSize,
		DrawFunc: ioprogress.DrawTerminalf(os.Stderr, fmtfunc),
	}

	n, err := io.Copy(out, progressR)
	if err != nil {
		log.Fatal(err, n)
	}
}

func getProjectName(token, projectId string) string {
	url := basespaceApiUrl + "/projects/" + projectId + "/?access_token="

	resp, err := http.Get(url + token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading URL: %s\n\n", url)
		os.Exit(1)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing reading result: %s\n\n", string(body))
		os.Exit(1)
	}

	var f map[string]interface{}

	if err = json.Unmarshal(body, &f); err != nil {
		fmt.Fprintf(os.Stderr, "Error processing JSON: %s\n\n", string(body))
		os.Exit(1)
	}

	return (f["Response"].(map[string]interface{}))["Name"].(string)
}

func getSampleName(token, sampleId string) string {
	url := basespaceApiUrl + "/samples/" + sampleId + "/?access_token="

	resp, err := http.Get(url + token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading URL: %s\n\n", url)
		os.Exit(1)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing reading result: %s\n\n", string(body))
		os.Exit(1)
	}

	var f map[string]interface{}

	if err = json.Unmarshal(body, &f); err != nil {
		fmt.Fprintf(os.Stderr, "Error processing JSON: %s\n\n", string(body))
		os.Exit(1)
	}

	return (f["Response"].(map[string]interface{}))["Name"].(string)
}

func downloadProject(token, projectId string, dryrun bool) {
	fmt.Fprintf(os.Stderr, "Project: [%s] %s\n", projectId, getProjectName(token, projectId))

	offset := 0
	total := 0

	for total == 0 || offset < total {
		url := fmt.Sprintf("%s/projects/%s/samples?Offset=%d&access_token=", basespaceApiUrl, projectId, offset)

		resp, err := http.Get(url + token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading URL: %s\n\n", url)
			os.Exit(1)
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing reading result: %s\n\n", string(body))
			os.Exit(1)
		}

		var f map[string]interface{}

		if err = json.Unmarshal(body, &f); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing JSON: %s\n\n", string(body))
			os.Exit(1)
		}

		for k, v := range f {
			if k == "Response" {
				for k2, v2 := range v.(map[string]interface{}) {
					if k2 == "Items" {
						items := v2.([]interface{})
						for i := range items {
							v3 := items[i].(map[string]interface{})
							sampleId := v3["Id"].(string)
							sampleName := v3["Name"].(string)
							downloadSample(token, sampleId, sampleName, "  ", dryrun)
						}
					}
				}
				total = int(v.(map[string]interface{})["TotalCount"].(float64))
				displayed := int(v.(map[string]interface{})["DisplayedCount"].(float64))
				offset += displayed
			}
		}
	}
}
