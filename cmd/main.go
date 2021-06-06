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
		DisableColors:          true,
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
	st := make(chan bool)

	go ticker(st)
	actors, _ := spammy.CreateRandomActors(DefaultActorCount, *concurrent)
	fmt.Printf("\nCreated %d actors\n", len(actors))

	actors, _ = spammy.LoadActors(ap.IRI(*serv), *concurrent)
	fmt.Printf("\nLoaded %d actors\n", len(actors))

	objects, _ := spammy.CreateRandomObjects(DefaultObjectCount, *concurrent, actors)
	fmt.Printf("\nCreated %d objects\n", len(objects))

	objects, _ = spammy.LoadObjects(ap.IRI(*serv), *concurrent)
	fmt.Printf("\nLoaded %d objects\n", len(objects))

	activities, _ := spammy.CreateRandomActivities(DefaultActivitiesCount, *concurrent, objects, actors)
	fmt.Printf("\nExecuted %d activities\n", len(activities))
	st <- true
}

func ticker(stopCh <- chan bool) {
	stop := false
	for {
		go func() {
			fmt.Printf(".")
			select {
			case stop = <-stopCh:
				fmt.Printf("\n")
				return
			}
		}()
		if stop {
			break
		}
		time.Sleep(700*time.Millisecond)
	}
}
