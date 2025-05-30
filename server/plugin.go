package main

import (
	"flag"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/adfinis-forks/mattermost-plugin-memes/server/meme"
	"github.com/adfinis-forks/mattermost-plugin-memes/server/memelibrary"
)

const memeCommand = "meme"

func resolveMeme(input string) (*meme.Template, []string, error) {
	if template, text := memelibrary.PatternMatch(input); template != nil {
		return template, text, nil
	}
	parts, err := shellquote.Split(input)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse input")
	}
	if template := memelibrary.Template(parts[0]); template != nil {
		return template, parts[1:], nil
	}
	return nil, nil, fmt.Errorf("i don't know this meme")
}

func demo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	out := fs.String("out", "", "output path to write the meme to")
	if err := fs.Parse(args); err != nil {
		return errors.Wrap(err, "failed to parse arguments")
	}

	input := fs.Args()
	if len(input) != 1 {
		return fmt.Errorf("specify an input string")
	}

	template, text, err := resolveMeme(input[0])
	if err != nil {
		return err
	}

	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return errors.Wrap(err, "failed to create output file")
		}
		defer f.Close() //nolint:errcheck

		img, err := template.Render(text)
		if err != nil {
			return errors.Wrap(err, "failed to render template")
		}
		if err := jpeg.Encode(f, img, &jpeg.Options{
			Quality: 100,
		}); err != nil {
			return errors.Wrap(err, "failed to encode image")
		}
	}

	return nil
}

func serveTemplateJPEG(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateName := vars["name"]

	template := memelibrary.Template(templateName)
	if template == nil {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query()
	text := query["text"]

	img, err := template.Render(text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	if err := jpeg.Encode(w, img, &jpeg.Options{
		Quality: 90,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type Plugin struct {
	plugin.MattermostPlugin

	router *mux.Router
}

func (p *Plugin) OnActivate() error {
	if p.API.GetConfig().ServiceSettings.SiteURL == nil {
		p.API.LogError("SiteURL must be set. Some features will operate incorrectly if the SiteURL is not set. See documentation for details: http://about.mattermost.com/default-site-url")
	}
	p.router = mux.NewRouter()
	p.router.HandleFunc("/templates/{name}.jpg", serveTemplateJPEG).Methods("GET")
	if err := p.API.RegisterCommand(createMemesCommand()); err != nil {
		return errors.Wrapf(err, "failed to register %s command", memeCommand)
	}
	return nil
}

func createMemesCommand() *model.Command {
	availableMemes := getAvailableMemes()
	memes := model.NewAutocompleteData(memeCommand, "[meme-name]", "Create awesome Memes yourself!")

	for _, name := range availableMemes {
		currentMeme := model.NewAutocompleteData(name, "", fmt.Sprintf("sends %s meme", name))
		currentMeme.AddTextArgument("displays text on meme", "[text]", "")
		memes.AddCommand(currentMeme)
	}

	return &model.Command{
		Trigger:          memeCommand,
		AutoComplete:     true,
		AutoCompleteDesc: "Renders custom memes so you can express yourself with culture.",
		AutocompleteData: memes,
	}
}

func getAvailableMemes() []string {
	var availableMemes []string //nolint:prealloc
	for name, metadata := range memelibrary.Memes() {
		availableMemes = append(availableMemes, name)
		availableMemes = append(availableMemes, metadata.Aliases...)
	}
	sort.Strings(availableMemes)
	return availableMemes
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	siteURL := p.GetSiteURL()

	input := strings.TrimSpace(strings.TrimPrefix(args.Command, "/meme"))

	if input == "" || input == "help" {
		availableMemes := getAvailableMemes()
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeInChannel,
			Text: `You can get started meming in one of two ways:

If your meme has well-defined phrasing, you can just type it:

` + "`" + `/meme brace yourself. memes are coming.` + "`" + `

If your meme doesn't have well-defined phrasing or you want more control, you can name a meme, then follow it with text to fill the slots:

` + "`" + `/meme brace-yourself "brace yourself." "memes are coming."` + "`" + `

In either case...

![brace-yourself](` + siteURL + `/plugins/memes/templates/brace-yourselves.jpg?text=brace+yourself.&text=memes+are+coming.)

Available memes: ` + strings.Join(availableMemes, ", "),
		}, nil
	}

	template, text, err := resolveMeme(input)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "error resolving meme", nil, err.Error(), http.StatusInternalServerError)
	}

	queryString := ""
	for _, t := range text {
		if queryString == "" {
			queryString += "?"
		} else {
			queryString += "&"
		}
		queryString += "text=" + url.QueryEscape(t)
	}

	resp := &model.CommandResponse{
		ResponseType: model.CommandResponseTypeInChannel,
		Text:         "![" + template.Name + "](" + siteURL + "/plugins/memes/templates/" + template.Name + ".jpg" + queryString + ")",
	}
	return resp, nil
}

func (p *Plugin) GetSiteURL() string {
	siteURL := ""
	ptr := p.API.GetConfig().ServiceSettings.SiteURL
	if ptr != nil {
		siteURL = *ptr
	}
	return siteURL
}

func main() {
	if len(os.Args) > 1 {
		if err := demo(os.Args[1:]); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		plugin.ClientMain(&Plugin{})
	}
}
