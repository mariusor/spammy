package spammy

import (
	"context"
	"sync"
	"time"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"golang.org/x/sync/errgroup"
)

type loader struct {
	f     *client.C
	queue chan ap.IRI
	res   chan ap.Item
	done  chan bool
}

func (l loader) loadFn (ctx context.Context, i ap.IRI) func() error {
	return func() error {
		dtx, cancelFn := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancelFn()

		ob, err := l.f.CtxLoadIRI(dtx, i)
		if err != nil {
			return err
		}
		var (
			next ap.Item
			items ap.ItemCollection
		)
		if ob.GetType() == ap.OrderedCollectionType {
			ap.OnOrderedCollection(ob, func(col *ap.OrderedCollection) error {
				// NOTE(marius): this wastes a request
				next = col.First
				return nil
			})
		}
		if ob.GetType() == ap.OrderedCollectionPageType {
			ap.OnOrderedCollectionPage(ob, func(col *ap.OrderedCollectionPage) error {
				next = col.Next
				items = col.OrderedItems
				return nil
			})
		}
		if items.Count() > 0 {
			for _, it := range items.Collection() {
				l.res <- it
			}
		}
		if next != nil && !next.GetLink().Equals(i, false) {
			l.queue <- next.GetLink()
		} else {
			l.done <- true
		}
		return nil
	}
}

func (l loader) wait(ctx context.Context, cancelFn func()) (map[ap.IRI]ap.Item, []error) {
	result := make(map[ap.IRI]ap.Item)
	errors := make([]error, 0)

	g := new(errgroup.Group)
	m := sync.Mutex{}

exit:
	for {
		select {
		case done := <-l.done:
			if done {
				cancelFn()
				break exit
			}
		case i := <-l.queue:
			g.Go(l.loadFn(ctx, i))
		case it := <-l.res:
			m.Lock()
			if it.GetType() != ap.TombstoneType {
				if _, ok := result[it.GetLink()]; !ok {
					result[it.GetLink()] = it
				}
			}
			m.Unlock()
		}
	}
	if err := g.Wait(); err != nil {
		errors = append(errors, err)
	}
	return result, errors
}

func load(iri ap.IRI, concurrent int) (map[ap.IRI]ap.Item, []error) {
	l := loader {
		f:     client.New(client.SkipTLSValidation(true), client.SetErrorLogger(ErrFn), client.SetInfoLogger(InfFn)),
		queue: make(chan ap.IRI, concurrent),
		res:   make(chan ap.Item),
		done:  make(chan bool, 1),
	}

	ctx, cancelFn := context.WithCancel(context.TODO())
	defer cancelFn()

	l.queue <- iri

	return l.wait(ctx, cancelFn)
}
