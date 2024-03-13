package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/mod/semver"
)

var (
	modsFolderPath = flag.String("modsFolder", "", "Path to mods folder")
)

const (
	modrinthAPI = "https://api.modrinth.com/v2/projects"
)

type mod struct {
	fname   string
	modid   string
	version string
}

func main() {
	flag.Parse()
	if *modsFolderPath == "" {
		fmt.Println("Mods folder path not set")
		return
	}
	mods := detectMods(path.Clean(*modsFolderPath))
	lookupVersions(mods)
	fmt.Println(writeTable(mods))
}

func writeTable(mods []mod) string {
	modsTable := table.NewWriter()
	modsTable.AppendHeader(table.Row{"Filename", "ID", "Version"})
	sort.Slice(mods, func(i, j int) bool {
		verc := strings.Compare(mods[i].version, mods[j].version)
		if verc == 0 {
			return strings.Compare(mods[i].modid, mods[j].modid) < 0
		}
		return verc < 0
	})
	for _, m := range mods {
		modsTable.AppendRow(table.Row{m.fname, m.modid, m.version})
	}
	modsTable.Style().Box = table.StyleBoxLight
	return modsTable.Render()
}

func lookupVersions(mods []mod) {
	IDs := []string{}
	for _, m := range mods {
		IDs = append(IDs, m.modid)
	}
	IDsJSON := noerr(json.Marshal(IDs))
	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	urlValues := url.Values{}
	urlValues.Add("ids", string(IDsJSON))
	resp := []struct {
		ID           string   `json:"slug"`
		GameVersions []string `json:"game_versions"`
	}{}
	must(json.NewDecoder(noerr(httpClient.Get(modrinthAPI + "?" + urlValues.Encode())).Body).Decode(&resp))
	for _, r := range resp {
		v := findTopVerison(r.GameVersions)
		for i, m := range mods {
			if m.modid == r.ID {
				mods[i].version = v
			}
		}
	}
}

func findTopVerison(versions []string) string {
	filtered := []string{}
	for _, v := range versions {
		vv := "v" + v
		if semver.IsValid(vv) {
			filtered = append(filtered, vv)
		}
	}
	semver.Sort(filtered)
	if len(filtered) == 0 {
		return "no versions"
	}
	return filtered[len(filtered)-1]
}

func detectMods(dirPath string) []mod {
	ret := []mod{}
	modsFiles := noerr(os.ReadDir(dirPath))
	for _, modFile := range modsFiles {
		if modFile.IsDir() {
			continue
		}
		if !strings.HasSuffix(modFile.Name(), ".jar") {
			continue
		}
		modPath := path.Join(*modsFolderPath, modFile.Name())
		modZip, err := zip.OpenReader(modPath)
		if err != nil {
			fmt.Println("Error opening ", modPath, ": ", err.Error())
			continue
		}
		modID := extractModID(modZip)
		if modID == "" {
			continue
		}
		m := mod{
			fname: modFile.Name(),
			modid: modID,
		}
		ret = append(ret, m)
	}
	return ret
}

func extractModID(modZip *zip.ReadCloser) string {
	for _, zipFile := range modZip.File {
		if zipFile.Name != "fabric.mod.json" {
			continue
		}
		modInfo := map[string]any{}
		must(json.NewDecoder(noerr(zipFile.Open())).Decode(&modInfo))
		modID, ok := modInfo["id"].(string)
		if !ok {
			continue
		}
		return modID
	}
	return ""
}

func must(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func noerr[T any](ret T, err error) T {
	must(err)
	return ret
}
