package src

import (
	"fmt"
	"grape/src/common/config"
	"grape/src/common/dto/request"
	"grape/src/common/helper"
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
	links  map[string]string
}

func NewIndexService(s *service.CommonService) *IndexService {
	return (&IndexService{config: s.Config, links: make(map[string]string)}).fill().sync()
}

func (c *IndexService) compress(file string) string {
	src := filepath.Join(c.config.Vrchat.Dst, filepath.Base(file))
	dst := filepath.Join(c.config.Vrchat.Dst, "thumbnails", filepath.Base(file))

	helper.Thumbnail(src, dst, 30)

	return file
}

func (c *IndexService) fill() *IndexService {
	entries, _ := os.ReadDir(c.config.Vrchat.Dst)
	for _, f := range entries {
		if f.IsDir() {
			continue
		}

		file := filepath.Join(filepath.Base(c.config.Vrchat.Dst), f.Name())
		c.links[helper.Btoa(file)] = c.compress(file)
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
					c.links[helper.Btoa(file)] = c.compress(file)
				}
			}
		}
	}

	return c
}

func (c *IndexService) Copy(query string) error {
	cmds := []*exec.Cmd{exec.Command("xsel", "-ib"), exec.Command("wl-copy")}

	for _, cmd := range cmds {
		cmd.Stdin = strings.NewReader(query)
		if err := cmd.Run(); err != nil {
			fmt.Printf("%v\n", err)
			return err
		}
	}

	return nil
}

func (c *IndexService) FindAll(dto *request.PageDto) interface{} {
	keys := slices.Collect(maps.Values(c.sync().links))
	slices.Sort(keys)
	slices.Reverse(keys)
	return gin.H{"files": dto.Paginate(keys)}
}

func (c *IndexService) Delete(id string) interface{} {
	if file, ok := c.links[id]; ok {
		delete(c.links, id)

		os.Remove(filepath.Join(c.config.Vrchat.Dst, filepath.Base(file)))
		os.Remove(filepath.Join(c.config.Vrchat.Dst, "thumbnails", filepath.Base(file)))
	}

	return gin.H{}
}
