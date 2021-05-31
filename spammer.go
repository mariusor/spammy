package spammy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/go-ap/errors"
	h "github.com/go-ap/handlers"
	"github.com/mariusor/spammy/internal/names"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"hawx.me/code/indieauth"
)

const (
	Actors    h.CollectionType = "actors"
	DefaultPw                  = "asd"
)

var (
	httpClient = http.DefaultClient
	ServiceAPI = ap.IRI("https://fedbox.local")

	OAuthKey    = "aa52ae57-6ec6-4ddd-afcc-1fcbea6a29c0"
	OAuthSecret = "asd"

	Application *ap.Actor = nil
	ErrFn       func(c ...client.Ctx) client.LogFn
	InfFn       func(c ...client.Ctx) client.LogFn

	availableExtensions = [...]string{
		// text
		"html",
		"txt",
		"md",
		// document?
		"svg",
		// image
		"jpg",
		"png",
		// audio
		"mp3",
		// video
		"webm",
	}
)

func init() {
	data.Walk("data/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			ff, err := data.Open(path)
			if err != nil {
				return err
			}
			defer ff.Close()

			data, err := ioutil.ReadAll(ff)
			if err != nil {
				return err
			}
			contentType, _, _ := mime.ParseMediaType(http.DetectContentType(data))
			if validRandomFiles[contentType] == nil {
				validRandomFiles[contentType] = make([][]byte, 0)
			}
			validRandomFiles[contentType] = append(validRandomFiles[contentType], data)
		}
		return nil
	})

	if httpClient.Transport == nil {
		httpClient.Transport = http.DefaultTransport
	}
	if tr, ok := httpClient.Transport.(*http.Transport); ok {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = new(tls.Config)
		}
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
}

var validRandomFiles = make(map[string][][]byte)

func getObjectTypes(data []byte) (ap.ActivityVocabularyType, ap.MimeType) {
	contentType := http.DetectContentType(data)
	var objectType ap.ActivityVocabularyType

	contentType, _, _ = mime.ParseMediaType(contentType)
	switch contentType {
	case "text/html", "text/markdown", "text/plain":
		objectType = ap.NoteType
		if len(data) > 600 {
			objectType = ap.ArticleType
		}
		if bytes.Contains(data, []byte{'<', 's', 'v', 'g'}) {
			objectType = ap.DocumentType
			contentType = "image/svg+xml"
		}
	case "image/svg+xml":
		objectType = ap.DocumentType
	case "video/webm":
		fallthrough
	case "video/mp4":
		objectType = ap.VideoType
	case "audio/mp3":
		objectType = ap.AudioType
	case "image/png":
		fallthrough
	case "image/jpg":
		objectType = ap.ImageType
	}
	return objectType, ap.MimeType(contentType)
}

func GetRandomItemFromMap(m map[ap.IRI]ap.Item) ap.Item {
	if len(m) == 0 {
		return nil
	}
	pos := rand.Int() % len(m)
	cnt := 0
	for _, it := range m {
		if cnt == pos {
			return it
		}
		cnt++
	}
	return nil
}

func getRandomContentByMimeType(mimeType ap.MimeType) []byte {
	if validArray, ok := validRandomFiles[string(mimeType)]; ok {
		return validArray[rand.Int()%len(validArray)]
	}
	return nil
}

func getRandomContent() []byte {
	validArray := make([][]byte, 0)
	for _, files := range validRandomFiles {
		for _, file := range files {
			validArray = append(validArray, file)
		}
	}
	return validArray[rand.Int()%len(validArray)]
}

func getRandomName() []byte {
	return []byte(names.GetRandomName(10))
}

func RandomActor(parent ap.Item) ap.Item {
	act := new(ap.Actor)
	act.Name = ap.NaturalLanguageValues{
		{ap.NilLangRef, getRandomName()},
	}
	act.PreferredUsername = act.Name
	act.Type = ap.PersonType
	act.AttributedTo = parent
	act.Icon = RandomImage("image/png", parent.GetLink())
	return act
}

func RandomImage(mime ap.MimeType, parent ap.Item) ap.Item {
	img := new(ap.Image)
	img.Type = ap.ImageType
	img.MediaType = mime
	img.AttributedTo = parent

	data := getRandomContentByMimeType(mime)
	buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))
	base64.RawStdEncoding.Encode(buf, data)
	img.Content = ap.NaturalLanguageValues{
		{ap.NilLangRef, buf},
	}
	return img
}

func RandomObject(parent ap.Item) ap.Item {
	data := getRandomContent()
	typ, mime := getObjectTypes(data)

	ob := new(ap.Object)
	ob.Type = typ
	ob.MediaType = mime
	ob.AttributedTo = parent

	if !strings.Contains(string(mime), "text") {
		buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(data)))
		base64.RawStdEncoding.Encode(buf, data)
		data = buf
	} else {
		ob.Summary = ap.NaturalLanguageValues{
			{ap.NilLangRef, data[:bytes.Index(data, []byte{'.'})]},
		}
	}
	ob.Content = ap.NaturalLanguageValues{
		{ap.NilLangRef, data},
	}

	return ob
}

var validForActorActivityTypes = ap.ActivityVocabularyTypes{
	ap.UpdateType,
	ap.LikeType,
	ap.DislikeType,
	ap.FlagType,
	ap.BlockType,
	ap.FollowType,
	ap.IgnoreType,
}

var validForObjectActivityTypes = ap.ActivityVocabularyTypes{
	ap.UpdateType,
	ap.LikeType,
	ap.DislikeType,
	ap.DeleteType,
	ap.FlagType,
	ap.BlockType,
	ap.FollowType,
	ap.IgnoreType,
}

var validForActivityActivityTypes = ap.ActivityVocabularyTypes{
	ap.UndoType,
}

var validActivityTypes = append(validForObjectActivityTypes[:], validForActivityActivityTypes[:]...)

func getActivityTypeByObject(ob ap.Item) ap.ActivityVocabularyType {
	if ob != nil {
		return validForObjectActivityTypes[rand.Int()%len(validForObjectActivityTypes)]
	}
	if ap.ActivityTypes.Contains(ob.GetType()) {
		return validForActivityActivityTypes[rand.Int()%len(validForActivityActivityTypes)]
	}
	if ap.ActorTypes.Contains(ob.GetType()) {
		return validForActorActivityTypes[rand.Int()%len(validForActorActivityTypes)]
	}
	return validForObjectActivityTypes[rand.Int()%len(validForObjectActivityTypes)]
}

func getRandomReason() []byte {
	return []byte("A random reason for a stupid activity")
}

func RandomActivity(ob ap.Item, parent ap.Item) *ap.Activity {
	act := new(ap.Activity)
	act.Type = getActivityTypeByObject(ob)
	if ob != nil {
		act.Object = ob
	}
	act.AttributedTo = parent
	act.Actor = parent
	act.To = ap.ItemCollection{ServiceAPI, ap.PublicNS}

	if typesNeedReasons.Contains(act.Type) {
		act.Content = ap.NaturalLanguageValues{
			{ap.NilLangRef, getRandomReason()},
		}
		act.Summary = ap.NaturalLanguageValues{
			{ap.NilLangRef, getRandomReason()},
		}
	}

	return act
}

func LoadApplication(key string) error {
	f := client.New(client.SkipTLSValidation(true), client.SetErrorLogger(ErrFn), client.SetInfoLogger(InfFn))
	actors, err := f.Object(context.TODO(), SelfIRI(key))
	if err != nil {
		return err
	}
	err = ap.OnActor(actors, func(a *ap.Actor) error {
		Application = a
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func self() ap.Actor {
	return *Application
}

func SelfIRI(key string) ap.IRI {
	return Actors.IRI(ServiceAPI).AddPath(key)
}

func config(act *ap.Actor) oauth2.Config {
	conf := oauth2.Config{
		ClientID:     OAuthKey,
		ClientSecret: OAuthSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/oauth/authorize", ServiceAPI),
			TokenURL: fmt.Sprintf("%s/oauth/token", ServiceAPI),
		},
	}

	if act == nil {
		act = Application
	}
	if act != nil {
		endpoints := act.Endpoints
		if endpoints != nil {
			conf.Endpoint.AuthURL = endpoints.OauthAuthorizationEndpoint.GetLink().String()
			conf.Endpoint.TokenURL = endpoints.OauthTokenEndpoint.GetLink().String()
		}
		if act.URL != nil {
			conf.RedirectURL = act.URL.GetLink().String()
		}
	}

	return conf
}

func C2SSign(ctx context.Context, act *ap.Actor) client.RequestSignFn {
	if act == nil {
		return func(r *http.Request) error { return nil }
	}
	config := config(act)
	return func(req *http.Request) error {
		// set a custom http client to be used by the OAuth2 package, in our case, it has InsecureTLSCheck disabled
		//ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

		dtx, cancelFn := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancelFn()

		tok, err := config.PasswordCredentialsToken(dtx, act.GetID().String(), DefaultPw)
		if err != nil {
			return err
		}
		tok.SetAuthHeader(req)
		return nil
	}
}

func setSignFn(ctxt context.Context, f *client.C, activity ap.Item) error {
	return ap.OnActivity(activity, func(a *ap.Activity) error {
		if a.Actor == nil {
			return errors.Newf("Invalid actor when trying to sign C2S request")
		}
		actor, err := ap.ToActor(a.Actor)
		if actor != nil {
			f.SignFn(C2SSign(ctxt, actor))
		}
		return err
	})
}

func ExecActivity(ctx context.Context, f *client.C, activity ap.Item) (ap.Item, error) {
	dtx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	err := setSignFn(ctx, f, activity)
	if err != nil {
		return nil, err
	}

	ap.OnActivity(activity, func(act *ap.Activity) error {
		act.Actor = ap.FlattenToIRI(act.Actor)
		act.Object = ap.FlattenProperties(act.Object)
		return nil
	})
	iri, ff, err := f.ToOutbox(dtx, activity)
	if err != nil {
		return nil, err
	}
	if len(iri) > 0 {
		return f.Object(ctx, iri)
	}
	fmt.Printf("%v", ff)
	return nil, nil
}

type AuthorizeData struct {
	Code  string
	State string
}

func CreateActorActivity(ctx context.Context, f *client.C, ob ap.Item) (ap.Item, error) {
	a, err := CreateActivity(ctx, f, ob)
	if err != nil {
		return nil, err
	}

	self, _ := ap.ToActor(ob)
	config := config(self)
	config.Scopes = []string{"anonUserCreate"}

	res, err := f.CtxGet(ctx, config.AuthCodeURL(
		"spammy//test##",
		oauth2.SetAuthURLParam("actor", a.GetLink().String()),
	))
	if err != nil {
		return nil, err
	}

	var body []byte
	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		incoming, e := errors.UnmarshalJSON(body)
		var errs []error
		if e == nil {
			errs = make([]error, len(incoming))
			for i := range incoming {
				errs[i] = incoming[i]
			}
		} else {
			errs = []error{errors.WrapWithStatus(res.StatusCode, errors.Newf(""), "invalid response")}
		}
		//ErrFn()("errors: %s", errs)
		return nil, errs[0]
	}
	d := AuthorizeData{}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	if d.Code == "" {
		return nil, err
	}

	// pos
	pwChURL := fmt.Sprintf("%s/oauth/pw", ServiceAPI)
	u, _ := url.Parse(pwChURL)
	q := u.Query()
	q.Set("s", d.Code)
	u.RawQuery = q.Encode()
	form := url.Values{}

	form.Add("pw", DefaultPw)
	form.Add("pw-confirm", DefaultPw)

	pwChRes, err := http.Post(u.String(), "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if body, err = ioutil.ReadAll(pwChRes.Body); err != nil {
		return nil, err
	}
	if pwChRes.StatusCode != http.StatusOK {
		return nil, err
	}
	return a, err
}

func CreateActivity(ctx context.Context, f *client.C, ob ap.Item) (ap.Item, error) {
	create := ap.Create{
		Type:   ap.CreateType,
		Object: ob,
		To:     ap.ItemCollection{ServiceAPI, ap.PublicNS},
	}
	err := ap.OnObject(ob, func(o *ap.Object) error {
		if o.AttributedTo != nil {
			create.Actor, _ = ap.ToActor(o.AttributedTo)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = setSignFn(ctx, f, create)
	if err != nil {
		return nil, err
	}

	err = ap.OnActivity(create, func(act *ap.Activity) error {
		act.Actor = ap.FlattenToIRI(act.Actor)
		act.Object = ap.FlattenProperties(act.Object)
		return nil
	})
	if err != nil {
		return nil, err
	}

	dtx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	iri, final, err := f.ToOutbox(dtx, create)
	if err != nil {
		return final, err
	}

	it, err := f.Object(dtx, iri)
	if err != nil {
		return nil, err
	}

	if j, err := json.Marshal(it); err == nil {
		InfFn()("Activity: %s\n", j)
	}
	return final, nil
}

var MaxConcurrency = 1

func exec(cnt int, actFn func(context.Context, *client.C, ap.Item) (ap.Item, error), itFn func() ap.Item) (map[ap.IRI]ap.Item, error) {
	result := make(map[ap.IRI]ap.Item)
	m := sync.Mutex{}

	for i := 0; i < cnt; i += MaxConcurrency {
		g, gtx := errgroup.WithContext(context.TODO())
		for j := i; j < i+MaxConcurrency && j < cnt; j++ {
			f := client.New(client.SkipTLSValidation(true), client.SetErrorLogger(ErrFn), client.SetInfoLogger(InfFn))
			g.Go(func() error {
				ob, err := actFn(gtx, f, itFn())
				if err == nil && ob != nil {
					m.Lock()
					defer m.Unlock()
					result[ob.GetLink()] = ob
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			//ErrFn()(err.Error())
		}
	}
	return result, nil
}

func CreateIndieAuthApplication(me *ap.Person) (ap.Item, error) {
	auth := config(me)
	authURL, err := url.Parse(auth.Endpoint.AuthURL)
	if err != nil {
		return nil, err
	}
	tokURL, err := url.Parse(auth.Endpoint.TokenURL)
	if err != nil {
		return nil, err
	}
	// first we get the configuration for our client
	config, _ := indieauth.Authentication("https://brutalinks.git", "https://brutalinks.git/auth/fedbox/callback")
	config.Client = httpClient

	endpoints := indieauth.Endpoints{
		Authorization: authURL,
		Token:         tokURL,
	}
	profile, err := config.Exchange(endpoints, "test")
	if err != nil {
		return nil, err
	}
	// FIXME(marius): this needs to load the profile proper
	return ap.IRI(profile), err
}

func CreateRandomActors(cnt int) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActorActivity, func() ap.Item {
		return RandomActor(self())
	})
}

func CreateRandomObjects(cnt int, actors map[ap.IRI]ap.Item) (map[ap.IRI]ap.Item, error) {
	return exec(cnt, CreateActivity, func() ap.Item {
		actor := GetRandomItemFromMap(actors)
		return RandomObject(actor)
	})
}

var typesNeedReasons = ap.ActivityVocabularyTypes{ap.BlockType, ap.FlagType, ap.IgnoreType}

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
		parent := GetRandomItemFromMap(actors)
		actRes, err := exec(cnt, ExecActivity, func() ap.Item {
			act := RandomActivity(iri, parent)
			act.CC = append(act.CC, self())
			return act
		})
		if err != nil {
			continue
		}
		for k, v := range actRes {
			result[k] = v
		}
	}
	return result, nil
}
