package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/mariusor/spammy"
	"github.com/peterbourgon/ff"
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

func printItems (items map[ap.IRI]ap.Item) {
	for _, it := range items {
		if j, err := json.Marshal(it); err == nil {
			fmt.Printf("%s: %s\n", it.GetType(), j)
		}
	}
}

func main() {
	fs := flag.NewFlagSet("spammy", flag.ExitOnError)
	var (
		concurrent = fs.Int("concurrent", spammy.MaxConcurrency, "The number of concurrent requests to try")
		key        = fs.String("client", "", "The application Uuid")
		secret     = fs.String("secret", "", "The application secret")
		serv       = fs.String("url", spammy.ServiceAPI.String(), "The FedBOX url to connect to")
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

	ff.Parse(fs, os.Args[1:])
	if serv != nil {
		spammy.ServiceAPI = ap.IRI(*serv)
	}

	spammy.ErrFn = errf
	spammy.InfFn = infof

	if *secret != "" {
		spammy.OAuthSecret = *secret
	}
	if *key == "" {
		errf()("We need an application OAuth2 key to continue")
		os.Exit(1)
	}

	spammy.MaxConcurrency = *concurrent
	spammy.OAuthKey = *key
	if err := spammy.LoadApplication(*key); err != nil {
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
	//printItems(actors)

	objects, _ := spammy.CreateRandomObjects(DefaultObjectCount, actors)
	//printItems(objects)

	for iri, actor := range actors {
		objects[iri] = actor
	}
	activities, _ := spammy.CreateRandomActivities(DefaultActivitiesCount, objects, actors)
	//printItems(activities)
	for iri, activity := range activities {
		objects[iri] = activity
	}
}
