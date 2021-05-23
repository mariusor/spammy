package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	h "github.com/go-ap/handlers"
	"github.com/mariusor/spammy"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)


var (
	logger = logrus.New()
	fedbox client.ActivityPub = nil
	ServiceAPI  = ap.IRI("https://fedbox.local")

	OAuthKey    = "aa52ae57-6ec6-4ddd-afcc-1fcbea6a29c0"
	OAuthSecret = "asd"
)

func config() oauth2.Config {
	return oauth2.Config{
		ClientID:     OAuthKey,
		ClientSecret: OAuthSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/oauth/authorize", ServiceAPI),
			TokenURL: fmt.Sprintf("%s/oauth/token", ServiceAPI),
		},
		RedirectURL: fmt.Sprintf("https://brutalinks.local/auth/fedbox/callback"),
	}
}
func SelfIRI() ap.IRI {
	return spammy.Actors.IRI(ServiceAPI).AddPath(OAuthKey)
}

func self() ap.Actor {
	self := SelfIRI()
	return ap.Application{
		ID:     self,
		Type:   ap.ApplicationType,
		Outbox: h.Outbox.IRI(self),
		Inbox:  h.Inbox.IRI(self),
	}
}
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
	config := config()
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

func ExecActivity(act ap.Item, parent ap.Item) (ap.Item, error) {
	iri, ff, err := fedbox.ToCollection(h.Outbox.IRI(parent), act)
	if err != nil {
		return nil, err
	}
	if len(iri) > 0 {
		return fedbox.LoadIRI(iri)
	}
	fmt.Printf("%v", ff)
	return nil, nil
}

func CreateActivity(ob ap.Item, parent ap.Item) (ap.Item, error) {
	create := ap.Create{
		Type:   ap.CreateType,
		Object: ob,
		To:     ap.ItemCollection{ServiceAPI, ap.PublicNS},
		Actor: parent,
	}
	iri, final, err := fedbox.ToCollection(h.Outbox.IRI(parent), create)
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

func exec(cnt int, actFn func(ap.Item, ap.Item) (ap.Item, error), itFn func() ap.Item) (map[ap.IRI]ap.Item, error) {
	result := make(map[ap.IRI]ap.Item)
	for i := 0; i < cnt; i++ {
		it := itFn()
		var parent ap.Item
		ap.OnObject(it, func(o *ap.Object) error {
			parent = o.AttributedTo
			return nil
		})
		ob, err := actFn(it, parent)
		if err != nil {
			errf()("Error: %s", err)
			break
		}
		result[ob.GetLink()] = ob
	}
	return result, nil
}

func CreateRandomActors(cnt int) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActivity, func() ap.Item {
		return  spammy.RandomActor(self())
	})
}

func CreateRandomObjects(cnt int, actors map[ap.IRI]ap.Item) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActivity, func() ap.Item {
		return spammy.RandomObject(self())
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
		parent := spammy.GetRandomItemFromMap(actors)
		actRes, err := exec(cnt, ExecActivity, func() ap.Item {
			act := spammy.RandomActivity(iri, parent)
			act.CC = append(act.CC, self())
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
	serv := flag.String("url", ServiceAPI.String(), "The FedBOX url to connect to")
	key := flag.String("client", OAuthKey, "The FedBOX application uuid")
	secret := flag.String("secret", OAuthSecret, "The FedBOX application secret")
	flag.Parse()
	if serv != nil {
		ServiceAPI = ap.IRI(*serv)
	}
	if key != nil {
		OAuthKey = *key
	}
	if secret != nil {
		OAuthSecret = *secret
	}
	fedbox = client.New(
		client.TLSConfigSkipVerify(),
		client.SignFn(C2SSign()),
		client.SetErrorLogger(errf),
		client.SetInfoLogger(infof),
	)
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
