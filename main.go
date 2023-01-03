package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tstromberg/nykya/pkg/nykya"
	"github.com/tstromberg/nykya/pkg/store"
	"github.com/writeas/go-writeas/v2"
)

var (
	dryRunFlag   = flag.Bool("dry-run", false, "dry-run mode (don't buy/sell anything)")
	instanceFlag = flag.String("instance", "https://write.as", "blog to post to")
	fromDirFlag  = flag.String("from-dir", ".", "directory to import from")
)

type Config struct {
	AccessToken string `json:"access_token"`
}

type Post struct {
	FrontMatter nykya.FrontMatter
	Content     string
}

func userConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %v", err)
	}

	path := filepath.Join(home, ".writeas", "user.json")
	log.Printf("reading token from %s ...", path)
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %v", err)
	}

	var c Config
	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	err = json.Unmarshal(bs, &c)
	return &c, err
}

func gatherPosts(path string) ([]Post, error) {
	log.Printf("gathering posts from %s ...", path)
	ps := []Post{}

	ctx := context.Background()
	items, err := store.Scan(ctx, path)
	if err != nil {
		return ps, fmt.Errorf("scan: %w", err)
	}
	for _, i := range items {
		if i.FrontMatter.Kind != "post" {
			continue
		}
		if i.FrontMatter.Draft {
			log.Printf("Skipping draft post: %s", i.ContentPath)
			continue
		}
		if len(i.Inline) == 0 {
			log.Printf("Skipping empty post: %s", i.ContentPath)
			continue
		}
		ps = append(ps, Post{i.FrontMatter, i.Inline})
	}

	return ps, err
}

func main() {
	flag.Parse()
	cfg, err := userConfig()
	if err != nil {
		log.Fatalf("failed to load writeas-cli config: %v", err)
	}

	cl := writeas.NewClientWith(writeas.Config{
		URL:   *instanceFlag + "/api",
		Token: cfg.AccessToken,
	})

	posts, err := gatherPosts(*fromDirFlag)
	if err != nil {
		log.Fatalf("find posts: %v", err)
	}

	failed := []string{}
	for _, p := range posts {
		log.Printf("Uploading post %q from %s (%d bytes)", p.FrontMatter.Title, p.FrontMatter.Date, len(p.Content))

		if *dryRunFlag {
			continue
		}

		pp := &writeas.PostParams{
			Title:   p.FrontMatter.Title,
			Content: p.Content,
			Created: &p.FrontMatter.Date.Time,
			Font:    "norm",
		}
		_, err = cl.CreatePost(pp)
		if err != nil {
			failed = append(failed, pp.Title)
			log.Printf("create failed: %v", err)
			log.Printf("failed content: %+v", pp)
		}
	}

	log.Printf("%d of %d posts failed to upload: %v", len(failed), len(posts), failed)
}
