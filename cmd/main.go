package main

import (
	"encoding/json"
	"flag"
	"fmt"
	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/mariusor/spammy"
	"github.com/sirupsen/logrus"
)


var (
	logger = logrus.New()
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

func main() {
	serv := flag.String("url", spammy.ServiceAPI.String(), "The FedBOX url to connect to")
	key := flag.String("client", spammy.OAuthKey, "The FedBOX application uuid")
	secret := flag.String("secret", spammy.OAuthSecret, "The FedBOX application secret")
	flag.Parse()
	if serv != nil {
		spammy.ServiceAPI = ap.IRI(*serv)
	}
	if key != nil {
		spammy.OAuthKey = *key
	}
	if secret != nil {
		spammy.OAuthSecret = *secret
	}

	spammy.FedBOX = client.New(
		client.SkipTLSValidation(true),
		client.SignFn(spammy.C2SSign()),
		client.SetErrorLogger(errf),
		client.SetInfoLogger(infof),
	)
	spammy.ErrFn = errf
	err := spammy.LoadApplication()
	if err != nil {
		errf()("Error: %s", err)
		return
	}

	printItems := func(items map[ap.IRI]ap.Item) {
		for _, it := range items {
			if j, err := json.Marshal(it); err == nil {
				fmt.Printf("%s: %s\n", it.GetType(), j)
			}
		}
	}
	actors, _ := spammy.CreateRandomActors(20)
	printItems(actors)
	objects, _ := spammy.CreateRandomObjects(100, actors)
	printItems(objects)
	activities, _ := spammy.CreateRandomActivities(50, objects, actors)
	printItems(activities)
}
