package main

import (
	"context"
	"encoding/json"
	"fmt"
	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	h "github.com/go-ap/handlers"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
)

const (
	ServiceAPI  = ap.IRI("https://fedbox.local")
	OAuthKey    = "aa52ae57-6ec6-4ddd-afcc-1fcbea6a29c0"
	OAuthSecret = "asd"

	Actors h.CollectionType = "actors"
)

var (
	logger = logrus.New()
	fedbox = client.New(
		client.TLSConfigSkipVerify(),
		client.SignFn(C2SSign()),
		client.SetErrorLogger(errf),
		client.SetInfoLogger(infof),
	)
	config = oauth2.Config{
		ClientID:     OAuthKey,
		ClientSecret: OAuthSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/oauth/authorize", ServiceAPI),
			TokenURL: fmt.Sprintf("%s/oauth/token", ServiceAPI),
		},
		RedirectURL: fmt.Sprintf("https://brutalinks.local/auth/fedbox/callback"),
	}
	selfIRI = Actors.IRI(ServiceAPI).AddPath(OAuthKey)
	self    = ap.Application{
		ID:     selfIRI,
		Type:   ap.ApplicationType,
		Outbox: h.Outbox.IRI(selfIRI),
		Inbox:  h.Inbox.IRI(selfIRI),
	}
)

func fields(c ...client.Ctx) logrus.Fields {
	cc := make(logrus.Fields)
	for _, ctx := range c {
		for k, v := range ctx {
			cc[k] = v
		}
	}
	return cc
}

func logFn(c ...client.Ctx) *logrus.Entry {
	return logger.WithFields(fields(c...))
}

func infof(c ...client.Ctx) client.LogFn {
	return logFn(c...).Infof
}

func errf(c ...client.Ctx) client.LogFn {
	return logFn(c...).Errorf
}

func C2SSign() client.RequestSignFn {
	var tok *oauth2.Token
	return func(req *http.Request) error {
		if tok == nil {
			var err error
			tok, err = config.PasswordCredentialsToken(context.Background(), fmt.Sprintf("oauth-client-app-%s", OAuthKey), config.ClientSecret)
			if err != nil {
				return err
			}
		}
		tok.SetAuthHeader(req)
		return nil
	}
}

func ExecActivity(act ap.Item) (ap.Item, error) {
	iri, ff, err := fedbox.ToCollection(h.Outbox.IRI(self), act)
	if err != nil {
		return nil, err
	}
	if len(iri) > 0 {
		return fedbox.LoadIRI(iri)
	}
	fmt.Printf("%v", ff)
	return nil, nil
}

func CreateActivity(ob ap.Item) (ap.Item, error) {
	create := ap.Create{
		Type:   ap.CreateType,
		Object: ob,
		To:     ap.ItemCollection{ServiceAPI, ap.PublicNS},
	}
	iri, final, err := fedbox.ToCollection(h.Outbox.IRI(self), create)
	if err != nil {
		return final, err
	}
	it, err := fedbox.LoadIRI(iri)
	if err != nil {
		return nil, err
	}
	if j, err := json.Marshal(it); err == nil {
		fmt.Printf("Activity: %s\n", j)
	}
	return final, nil
}

func exec(cnt int, actFn func(ap.Item) (ap.Item, error), itFn func() ap.Item) (map[ap.IRI]ap.Item, error) {
	result := make(map[ap.IRI]ap.Item)
	for i := 0; i < cnt; i++ {
		ob, err := actFn(itFn())
		if err != nil {
			errf()("Error: %s", err)
			continue
		}
		result[ob.GetLink()] = ob
	}
	return result, nil
}

func CreateRandomActors(cnt int) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActivity, RandomActor)
}

func CreateRandomObjects(cnt int, actors map[ap.IRI]ap.Item) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActivity, func() ap.Item {
		return RandomObject(self)
	})
}

func CreateRandomActivities(cnt int, objects map[ap.IRI]ap.Item, actors map[ap.IRI]ap.Item) (map[ap.IRI]ap.Item, error) {
	iris := make([]ap.IRI, len(objects))
	i := 0
	for iri, it := range objects {
		if it.GetType() == ap.TombstoneType {
			continue
		}
		iris[i] = iri
		i++
	}
	result := make(map[ap.IRI]ap.Item)
	for _, iri := range iris {
		actRes, err := exec(cnt, ExecActivity, func() ap.Item {
			act := RandomActivity(iri)
			act.CC = append(act.CC, getRandomItemFromMap(actors))
			return act
		})
		if err != nil {
			errf()("Error: %s", err)
			continue
		}
		for k, v := range actRes {
			result[k] = v
		}
	}
	return result, nil
}

func main() {
	printItems := func(items map[ap.IRI]ap.Item) {
		for _, it := range items {
			if j, err := json.Marshal(it); err == nil {
				fmt.Printf("%s: %s\n", it.GetType(), j)
			}
		}
	}
	actors, _ := CreateRandomActors(20)
	printItems(actors)
	objects, _ := CreateRandomObjects(100, actors)
	printItems(objects)
	activities, _ := CreateRandomActivities(50, objects, actors)
	printItems(activities)
}
