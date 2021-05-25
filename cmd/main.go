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

const (
	DefaultActorCount      = 20
	DefaultObjectCount     = 100
	DefaultActivitiesCount = 100
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

	spammy.FedBOX = client.New(client.SkipTLSValidation(true), client.SetErrorLogger(errf), client.SetInfoLogger(infof))
	spammy.ErrFn = errf
	if err := spammy.LoadApplication(); err != nil {
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
	actors, _ := spammy.CreateRandomActors(DefaultActorCount)
	printItems(actors)
	objects, _ := spammy.CreateRandomObjects(DefaultObjectCount, actors)
	printItems(objects)
	activities, _ := spammy.CreateRandomActivities(DefaultActivitiesCount, objects, actors)
	printItems(activities)
}
