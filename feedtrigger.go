// Package feedtrigger is a simple library which is aimed to handle new
// RSS/Atom entries by using a trigger function.
package feedtrigger

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mmcdole/gofeed"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/bbolt"
)

// FeedAction is a the main configuration struct.
type FeedAction struct {
	Store gokv.Store
	Feeds []Feed
	sync.Mutex
}

// NewItemAction is triggered, when new item is available.
type NewItemAction func(*gofeed.Item) error

// Feed to poll (Atom/RSS).
type Feed struct {
	URL           string
	OnNewRecord   NewItemAction
	RefreshPeriod time.Duration
}

// NewFeed returns a feed by URL with default refresh period of 1 minute.
func NewFeed(url string, action NewItemAction) *Feed {
	return &Feed{
		URL:           url,
		OnNewRecord:   action,
		RefreshPeriod: 1 * time.Minute,
	}
}

// FeedHead is the top item of the feed. It's needed for checking for updates
// on every poll.
type FeedHead struct {
	Title     string `json:"title,omitempty"`
	Updated   string `json:"last_updated,omitempty"`
	Published string `json:"published,omitempty"`
}

// New application builder.
func New(s gokv.Store, ff ...Feed) (*FeedAction, error) {
	var err error
	store := s
	if store == nil {
		store, err = bbolt.NewStore(bbolt.DefaultOptions)
		if err != nil {
			return nil, fmt.Errorf("bbolt.NewStore: %w", err)
		}
	}

	app := &FeedAction{
		Store: store,
		Feeds: ff,
	}

	return app, nil
}

// Run polling and processing loop.
func (a *FeedAction) Run(ctx context.Context) error {
	defer a.Store.Close()
	g, gctx := errgroup.WithContext(ctx)
	for _, f := range a.Feeds {
		f := f
		g.Go(func() error {
			// log.Printf("start polling %s", f.URL)
			err := a.run(gctx, f) // fail early
			if err != nil {
				return err
			}
			for range time.NewTicker(f.RefreshPeriod).C {
				// log.Printf("run on tick %s", f.URL)
				err := a.run(gctx, f)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}

	return g.Wait()
}

func (a *FeedAction) run(ctx context.Context, f Feed) error {
	feed, err := gofeed.NewParser().ParseURLWithContext(f.URL, ctx)
	if err != nil {
		return fmt.Errorf("fetching feed: %w", err)
	}
	zitem := feed.Items[0]

	var head FeedHead
	found, err := a.Store.Get(f.URL, &head)
	if err != nil {
		return fmt.Errorf("get from store: %w", err)
	}

	if !found { //first run
		a.Lock()
		err := a.Store.Set(f.URL, &FeedHead{
			Title:     zitem.Title,
			Updated:   zitem.Updated,
			Published: zitem.Published,
		})
		if err != nil {
			return fmt.Errorf("storing: %w", err)
		}
		a.Unlock()
		return nil
	}

	for i := 0; i < len(feed.Items); i++ {
		if head.Title != feed.Items[i].Title {
			err = f.OnNewRecord(feed.Items[i])
			if err != nil {
				return fmt.Errorf("trigger func: %w", err)
			}
		} else {
			break
		}
	}
	a.Lock()
	err = a.Store.Set(f.URL, &FeedHead{
		Title:     zitem.Title,
		Updated:   zitem.Updated,
		Published: zitem.Published,
	})
	if err != nil {
		return fmt.Errorf("storing head: %w", err)
	}
	a.Unlock()

	return nil
}

// Person from the feed.
type Person struct {
	*gofeed.Person
}

/// NewPerson gives us a builder pattern.
func NewPerson(p *gofeed.Person) *Person {
	return &Person{p}
}

// String interface implementation for Person.
func (p *Person) String() string {
	var parts []string
	for _, s := range []string{p.Name, p.Email} {
		if strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// LogAuthorAndLink of the new item.
func LogAuthorAndLink(i *gofeed.Item) error {
	author := "unknown author"
	if i.Author != nil {
		author = NewPerson(i.Author).String()
	}
	log.Printf("%s: %s", author, i.Link)
	return nil
}
