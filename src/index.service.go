package src

import (
	"grape/src/common/config"
	"grape/src/common/service"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

type IndexService struct {
	config *config.Config
	links  map[string]bool
}

func NewIndexService(s *service.CommonService) *IndexService {
	return (&IndexService{config: s.Config, links: make(map[string]bool)}).fill().sync()
}

func (c *IndexService) fill() *IndexService {
	entries, _ := os.ReadDir(c.config.Vrchat.Dst)
	for _, f := range entries {
		c.links[filepath.Join(filepath.Base(c.config.Vrchat.Dst), f.Name())] = true
	}

	return c
}

func (c *IndexService) sync() *IndexService {
	entries, _ := os.ReadDir(c.config.Vrchat.Src)

	for _, d := range entries {
		if d.IsDir() {
			entries, _ := os.ReadDir(filepath.Join(c.config.Vrchat.Src, d.Name()))

			for _, f := range entries {
				file := filepath.Join(filepath.Base(c.config.Vrchat.Dst), f.Name())

				if _, ok := c.links[file]; !ok {
					exec.Command("mv", filepath.Join(c.config.Vrchat.Src, d.Name(), f.Name()), filepath.Join(c.config.Vrchat.Dst, f.Name())).Run()
					c.links[file] = true
				}
			}
		}
	}

	return c
}

func (c *IndexService) Copy(query string) error {
	println("query=" + query)
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(query)
	return cmd.Run()
}

func (c *IndexService) Image() interface{} {
	keys := slices.Collect(maps.Keys(c.sync().links))
	slices.Sort(keys)
	slices.Reverse(keys)
	return gin.H{"files": keys}
}
