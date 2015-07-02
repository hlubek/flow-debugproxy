package pathmapper

import (
	"github.com/dfeyer/flow-debugproxy/config"
	"github.com/dfeyer/flow-debugproxy/errorhandler"
	"github.com/dfeyer/flow-debugproxy/logger"

	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var h = "%s"
var mapping = map[string]string{}

//PathMapper handle the mapping between real code and proxy
type PathMapper struct {
	Config *config.Config
}

//ApplyMappingToTextProtocol change file path in xDebug text protocol
func (p *PathMapper) ApplyMappingToTextProtocol(protocol []byte) []byte {
	commandParts := strings.Fields(fmt.Sprintf(h, protocol))
	command := commandParts[0]
	if command == "breakpoint_set" {
		file := commandParts[6]
		if p.Config.Verbose {
			logger.Info("Command: %s", logger.Colorize(command, "blue"))
		}
		fileMapping := p.mapPath(file)
		protocol = bytes.Replace(protocol, []byte(file), []byte("file://"+fileMapping), 1)
	}

	return protocol
}

//ApplyMappingToXML change file path in xDebug XML protocol
func (p *PathMapper) ApplyMappingToXML(xml []byte) []byte {
	r := regexp.MustCompile(`filename=["]?file://(\S+)/Data/Temporary/[^/]*/Cache/Code/Flow_Object_Classes/([^"]*)\.php`)
	var processedMapping = map[string]string{}

	for _, match := range r.FindAllStringSubmatch(string(xml), -1) {
		path := match[1] + "/Data/Temporary/" + p.Config.Context + "/Cache/Code/Flow_Object_Classes/" + match[2] + ".php"
		if _, ok := processedMapping[path]; ok == false {
			if originalPath, exist := mapping[path]; exist {
				if p.Config.VeryVerbose {
					logger.Info("Umpa Lumpa can help you, he know the mapping\n%s\n%s\n", logger.Colorize(">>> "+fmt.Sprintf(h, path), "yellow"), logger.Colorize(">>> "+fmt.Sprintf(h, p.getRealFilename(originalPath)), "green"))
				}
				processedMapping[path] = originalPath
			} else {
				originalPath = p.readOriginalPathFromCache(path)
				processedMapping[path] = originalPath
			}
		}
	}

	for path, originalPath := range processedMapping {
		path = p.getRealFilename(path)
		originalPath = p.getRealFilename(originalPath)
		xml = bytes.Replace(xml, []byte(path), []byte(originalPath), -1)
	}
	s := strings.Split(string(xml), "\x00")
	i, err := strconv.Atoi(s[0])
	errorhandler.PanicHandling(err)
	l := len(s[1])
	if i != l {
		xml = bytes.Replace(xml, []byte(strconv.Itoa(i)), []byte(strconv.Itoa(l)), 1)
	}

	return xml
}

//getRealFilename remove protocol from the given path
func (p *PathMapper) getRealFilename(path string) string {
	return strings.TrimPrefix(path, "file://")
}

func (p *PathMapper) mapPath(originalPath string) string {
	if strings.Contains(originalPath, "/Packages/") {
		parts := p.buildClassNameFromPath(originalPath)
		codeCacheFileName := parts[0] + "/Data/Temporary/" + p.Config.Context + "/Cache/Code/Flow_Object_Classes/" + parts[1] + ".php"
		realCodeCacheFileName := p.getRealFilename(codeCacheFileName)
		if _, err := os.Stat(realCodeCacheFileName); err == nil {
			return p.registerPathMapping(realCodeCacheFileName, originalPath)
		}
	}

	return originalPath
}

func (p *PathMapper) registerPathMapping(path string, originalPath string) string {
	dat, err := ioutil.ReadFile(path)
	errorhandler.PanicHandling(err)
	//check if file contains flow annotation
	if strings.Contains(string(dat), "@Flow\\") {
		if p.Config.Verbose {
			logger.Info("%s", "Our Umpa Lumpa take care of your mapping and they did a great job, they found a proxy for you:")
			logger.Info(">>> %s\n", path, "green")
		}

		if _, exist := mapping[path]; exist == false {
			mapping[path] = originalPath
		}
		return path
	}
	return originalPath
}

func (p *PathMapper) readOriginalPathFromCache(path string) string {
	dat, err := ioutil.ReadFile(path)
	errorhandler.PanicHandling(err)
	r := regexp.MustCompile(`(?m)^# PathAndFilename: (.*)$`)
	match := r.FindStringSubmatch(string(dat))
	//todo check if the match contain something
	originalPath := match[1]
	if p.Config.VeryVerbose {
		logger.Info("Umpa Lumpa need to work harder, need to reverse this one\n>>> %s\n>>> %s\n", logger.Colorize(fmt.Sprintf(h, path), "yellow"), logger.Colorize(fmt.Sprintf(h, originalPath), "green"))
	}
	p.registerPathMapping(path, originalPath)
	return originalPath
}

func (p *PathMapper) buildClassNameFromPath(path string) []string {
	// todo add support for PSR4
	r := regexp.MustCompile(`(.*?)/Packages/(.*?)/Classes/(.*).php`)
	match := r.FindStringSubmatch(path)
	basePath := match[1]
	r = regexp.MustCompile(`[\./]`)
	className := r.ReplaceAllString(match[3], "_")
	return []string{basePath, className}
}
