package main

import (
	"encoding/json"
	"flag"
	"fmt"
	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/mariusor/spammy"
	"github.com/sirupsen/logrus"
	"os"
	"time"
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
	var (
		key string
		secret string
	)
	logger.Formatter = &logrus.TextFormatter{
		ForceColors:            true,
		TimestampFormat:        time.StampMilli,
		FullTimestamp:          true,
		DisableSorting:         true,
		DisableLevelTruncation: false,
		PadLevelText:           true,
		QuoteEmptyFields:       false,
	}
	logger.Out = os.Stdout
	logger.Level = logrus.DebugLevel
	serv := flag.String("url", spammy.ServiceAPI.String(), "The FedBOX url to connect to")
	flag.StringVar(&key, "client", "", "The FedBOX application uuid")
	flag.StringVar(&secret, "secret", "", "The FedBOX application secret")
	flag.Parse()
	if serv != nil {
		spammy.ServiceAPI = ap.IRI(*serv)
	}

	spammy.FedBOX = client.New(client.SkipTLSValidation(true), client.SetErrorLogger(errf), client.SetInfoLogger(infof))
	spammy.ErrFn = errf
	spammy.InfFn = infof

	printItems := func(items map[ap.IRI]ap.Item) {
		for _, it := range items {
			if j, err := json.Marshal(it); err == nil {
				fmt.Printf("%s: %s\n", it.GetType(), j)
			}
		}
	}
	if secret != "" {
		spammy.OAuthSecret = secret
	}
	if key == "" {
		errf()("We need an application OAuth2 key to continue")
		os.Exit(1)
	}

	spammy.OAuthKey = key
	if err := spammy.LoadApplication(key); err != nil {
		errf()(err.Error())
		return
	}
	if false {
		app, err := spammy.CreateIndieAuthApplication(nil)
		if err != nil {
			errf()(err.Error())
			os.Exit(1)
		}
		if app != nil {
			spammy.Application, _ = ap.ToActor(app)
		}
	}

	actors, _ := spammy.CreateRandomActors(DefaultActorCount)
	printItems(actors)

	objects, _ := spammy.CreateRandomObjects(DefaultObjectCount, actors)
	printItems(objects)

	for iri, actor := range actors {
		objects[iri] = actor
	}
	activities, _ := spammy.CreateRandomActivities(DefaultActivitiesCount, objects, actors)
	printItems(activities)
}
