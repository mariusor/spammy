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
	"os"
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
	tok, err := config.PasswordCredentialsToken(context.Background(), fmt.Sprintf("oauth-client-app-%s", OAuthKey), config.ClientSecret)
	return func(req *http.Request) error {
		if tok == nil {
			tok, err = config.PasswordCredentialsToken(context.Background(), fmt.Sprintf("oauth-client-app-%s", OAuthKey), config.ClientSecret)
			if err != nil {
				return err
			}
		}
		tok.SetAuthHeader(req)
		return nil
	}
}

func main() {
	selfIRI := Actors.IRI(ServiceAPI).AddPath(OAuthKey)
	self := ap.Application{ID: selfIRI, Type: ap.ApplicationType, Outbox: h.Outbox.IRI(selfIRI), Inbox: h.Inbox.IRI(selfIRI)}

	o := ap.ObjectNew(ap.ArticleType)
	o.Content.Set(ap.NilLangRef, ap.Content("Hello, ActivityPub!"))

	a := ap.Create{
		Type:   ap.CreateType,
		Object: o,
		To:     ap.ItemCollection{ServiceAPI, ap.PublicNS},
	}

	iri, act, err := fedbox.ToCollection(h.Outbox.IRI(self), a)
	if err != nil {
		errf(client.Ctx{"err": err})("saving activity")
		os.Exit(1)
	}
	if iri != "" {
		ob, err := fedbox.LoadIRI(iri)
		if err != nil {
			errf(client.Ctx{"err": err})("loading created object")
			os.Exit(1)
		}
		j, _ := json.Marshal(ob)
		fmt.Printf("%s\n", j)
	}
	j, _ := json.Marshal(act)
	fmt.Printf("%s\n", j)
}
