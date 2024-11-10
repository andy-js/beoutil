// Copyright (c) 2020-2024 Andrew Stormont
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"beoutil/clients/beoremote"
	"beoutil/clients/beoremote/models"
	"beoutil/clients/deezer"
	deezerModels "beoutil/clients/deezer/models"

	"github.com/urfave/cli/v2"
)

func doFindProducts(c *cli.Context) error {
	if c.NArg() != 0 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	home := os.Getenv("HOME")
	if home == "" {
		return errors.New("HOME is not set")
	}
	_, _ = fmt.Fprintf(os.Stderr, "Scanning for products...\n")
	ctx, cancel := context.WithTimeout(context.Background(), c.Duration("timeout"))
	defer cancel()
	products, err := discoverProducts(ctx)
	if err != nil {
		return err
	}
	var b []byte
	b, err = json.Marshal(products)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(home, ".beoutil"), b, 0644)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "Found %d products.\n", len(products))
	return nil
}

func getCachedProducts() (map[models.Jid]*ProductDetails, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return nil, errors.New("HOME is not set")
	}
	b, err := os.ReadFile(filepath.Join(home, ".beoutil"))
	if os.IsNotExist(err) {
		return nil, errors.New("no products cached")
	}
	if err != nil {
		return nil, err
	}
	var products map[models.Jid]*ProductDetails
	if err = json.Unmarshal(b, &products); err != nil {
		return nil, err
	}
	return products, nil
}

type systemProduct struct {
	models.Product
	IPs []net.IP
}

func joinIPs(ips []net.IP) string {
	var b strings.Builder
	if len(ips) == 0 {
		return "-"
	}
	for i, ip := range ips {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(ip.String())
	}
	return b.String()
}

func getAllSystemProducts(ctx context.Context) (map[models.Jid]systemProduct, error) {
	cached, err := getCachedProducts()
	if err != nil {
		return nil, err
	}
	result := make(map[models.Jid]systemProduct)
	for _, c := range cached {
		for _, ip := range c.IPs {
			br := beoremote.NewClient(ip.String())
			var products []models.Product
			if products, err = br.BeoZone.GetSystemProducts(ctx); err != nil {
				continue
			}
			// Merge all the different product lists together.
			for _, p := range products {
				if _, ok := result[p.Jid]; !ok {
					result[p.Jid] = systemProduct{Product: p}
				}
			}
		}
	}
	// Add IP addresses to the list of products.
	for jid, r := range result {
		if c, ok := cached[jid]; ok {
			result[jid] = systemProduct{Product: r.Product, IPs: c.IPs}
		}
	}
	return result, nil
}

func doListProducts(c *cli.Context) error {
	if c.NArg() != 0 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	products, err := getAllSystemProducts(c.Context)
	if err != nil {
		return err
	}
	if len(products) > 0 {
		tw := new(tabwriter.Writer)
		tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
		_, _ = fmt.Fprintln(tw, "NAME\tROLE\tIP\tJID\tONLINE\tSTATE")
		for _, p := range products {
			role := "-"
			if p.Integrated != nil {
				switch p.Integrated.Role {
				case "integratedMaster":
					role = "master"
				case "integratedSlave":
					role = "slave"
				}
			}
			if role != "slave" {
				state := "-"
				if p.PrimaryExperience != nil {
					state = string(p.PrimaryExperience.State)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\t%s\n",
					p.FriendlyName, role, joinIPs(p.IPs), p.Jid, p.Online, state)
				if role == "master" {
					slaveState := "-"
					if products[p.Integrated.Jid].PrimaryExperience != nil {
						slaveState = string(products[p.Integrated.Jid].PrimaryExperience.State)
					}
					_, _ = fmt.Fprintf(tw, " + %s\t%s\t%s\t%s\t%t\t%s\n",
						products[p.Integrated.Jid].FriendlyName, "slave",
						joinIPs(products[p.Integrated.Jid].IPs), p.Integrated.Jid,
						products[p.Integrated.Jid].Online, slaveState)
				}
			}
		}
		_ = tw.Flush()
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "No products responded.\n")
	}
	return nil
}

func doAllStandby(c *cli.Context) error {
	if c.NArg() != 0 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	products, err := getCachedProducts()
	if err != nil {
		return err
	}
	for _, p := range products {
		for _, ip := range p.IPs {
			br := beoremote.NewClient(ip.String())
			var s models.PowerState
			s, err = br.BeoDevice.GetState(c.Context)
			if err != nil {
				continue
			}
			if s != models.PowerStateOn {
				break
			}
			err = br.BeoDevice.AllStandby(c.Context)
			if err == nil {
				break
			}
		}
	}
	return err
}

func doStandby(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoDevice.Standby(c.Context)
}

func doPowerOn(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoDevice.PowerOn(c.Context)
}

func doReboot(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoDevice.Reboot(c.Context)
}

func doGetVolume(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	v, err := br.BeoZone.GetVolume(c.Context)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "%d\n", v)
	return nil
}

func doSetVolume(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	v, err := strconv.Atoi(args.Get(1))
	if err != nil {
		return err
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.SetVolume(c.Context, v)
}

func doPause(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.Pause(c.Context)
}

func doPlay(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.Play(c.Context)
}

func doForward(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.Forward(c.Context)
}

func doBackward(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.Backward(c.Context)
}

func doStop(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.Stop(c.Context)
}

func doGetMuted(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	m, err := br.BeoZone.GetMuted(c.Context)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "%t\n", m)
	return nil
}

func doSetMuted(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	m, err := strconv.ParseBool(args.Get(1))
	if err != nil {
		return err
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.SetMuted(c.Context, m)
}

func doGetQueue(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	q, err := br.BeoZone.GetPlayQueue(c.Context, -200, 200)
	if err != nil {
		return err
	}
	if len(q.PlayQueueItem) > 0 {
		tw := new(tabwriter.Writer)
		tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
		// BeoSound Moment Bug: PlayNowId is
		// empty even though the queue isn't.
		if q.PlayNowId == "" {
			q.PlayNowId = q.PlayQueueItem[0].Id
		}
		_, _ = fmt.Fprintln(tw, "PTR\tPLID\tTRACK\tARTIST")
		for _, qi := range q.PlayQueueItem {
			marker := ""
			if qi.Id == q.PlayNowId {
				marker = "------>"
			}
			id := strings.TrimPrefix(string(qi.Id), "plid-")
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				marker, id, qi.Track.Name, qi.Track.ArtistName)
		}
		_ = tw.Flush()
		repeat := "unknown"
		if q.Repeat == models.RepeatAll {
			repeat = "all"
		} else if q.Repeat == models.RepeatCurrentItem {
			repeat = "current"
		} else if q.Repeat == models.RepeatOff {
			repeat = "off"
		}
		random := "unknown"
		if q.Random == models.RandomRandom {
			random = "on"
		} else if q.Random == models.RandomOff {
			random = "off"
		}
		fmt.Printf("Repeat: %s\tRandom: %s\n", repeat, random)
	} else {
		_, _ = fmt.Printf("Queue empty.\n")
	}
	return nil
}

func doClearQueue(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.ClearPlayQueue(c.Context)
}

func doRemoveQueueItem(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.RemoveQueueItem(c.Context, args.Get(1))
}

func doMoveQueueItem(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 3 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.MoveQueueItem(c.Context, args.Get(1), args.Get(2))
}

func doPlayQueueItem(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.PlayQueueItem(c.Context, args.Get(1))
}

func doSetRepeat(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	repeat := models.RepeatUnknown
	switch args.Get(1) {
	case "all":
		repeat = models.RepeatAll
	case "current":
		repeat = models.RepeatCurrentItem
	case "off":
		repeat = models.RepeatOff
	default:
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.SetQueueRepeat(c.Context, repeat)
}

func doSetRandom(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	random := models.RandomUnknown
	switch args.Get(1) {
	case "on":
		random = models.RandomRandom
	case "off":
		random = models.RandomOff
	default:
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.SetQueueRandom(c.Context, random)
}

func doGetSources(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	products, err := br.BeoZone.GetSystemProducts(c.Context)
	if err != nil {
		return err
	}
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PRODUCT NAME\tSOURCE NAME\tSOURCE ID\tLINKABLE")
	for _, product := range products {
		for _, source := range product.Source {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%t\n",
				product.FriendlyName, source.FriendlyName, source.Id, source.Linkable)
		}
	}
	_ = tw.Flush()
	return nil
}

func doSetActiveSource(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.PlaySource(c.Context, models.SourceID(args.Get(1)))
}

func doGetActiveSource(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	as, err := br.BeoZone.GetActiveSources(c.Context)
	if err != nil {
		return err
	}
	fmt.Println("Primary Experience:")
	if as.PrimaryExperience.Source.Id != "" {
		fmt.Printf("\tSource ID:\t%s\n", as.PrimaryExperience.Source.Id)
		fmt.Printf("\tSource Name:\t%s\n", as.PrimaryExperience.Source.FriendlyName)
		fmt.Printf("\tListeners:\n")
		for _, l := range as.PrimaryExperience.ListenerList.Listener {
			fmt.Printf("\t\t%s\n", l.Jid)
		}
	}
	fmt.Println("Active Sources:")
	if as.ActiveSources.PrimaryJid != "" {
		fmt.Printf("\tPrimary:\n")
		fmt.Printf("\t\tProduct ID:\t%s\n", as.ActiveSources.PrimaryJid)
		fmt.Printf("\t\tSource ID:\t%s\n", as.ActiveSources.Primary)
	}
	return nil
}

func doClearActiveSource(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.EndExperience(c.Context)
}

func doAddListener(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.AddListener(c.Context, models.Jid(args.Get(1)))
}

func doRemoveListener(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoZone.RemoveListener(c.Context, models.Jid(args.Get(1)))
}

func doSearchArtist(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	d := deezer.NewClient()
	artists, err := d.SearchArtist(c.Context, &deezer.SearchOptions{
		Q:     args.First(),
		Index: 0,
		Limit: c.Int("limit"),
	})
	if err != nil {
		return err
	}
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME")
	for _, a := range artists {
		_, _ = fmt.Fprintf(tw, "%d\t%s\n", a.ID, a.Name)
	}
	_ = tw.Flush()
	return nil
}

func doListAlbums(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	d := deezer.NewClient()
	iter := d.NewAlbumIter(args.First())
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTITLE\tTYPE\tEXPLICIT\tRELEASED")
	for {
		albums, err := iter.Next(c.Context)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		for _, a := range albums {
			_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%t\t%s\n", a.ID,
				a.Title, a.RecordType, a.ExplicitLyrics, a.ReleaseDate)
		}
	}
	if iter.Read() == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "No albums found.")
	} else {
		_ = tw.Flush()
	}
	return nil
}

func doListTracks(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		return os.ErrInvalid
	}
	d := deezer.NewClient()
	tracks, err := d.GetAlbumTracks(c.Context, args.First())
	if err != nil {
		return err
	}
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTITLE")
	for _, t := range tracks {
		_, _ = fmt.Fprintf(tw, "%d\t%s\n", t.ID, t.Title)
	}
	_ = tw.Flush()
	return nil
}

func getTrackImages(md5Image string) []models.Image {
	baseURL := "https://e-cdns-images.dzcdn.net/images/cover/" + md5Image
	return []models.Image{
		{
			URL:       path.Join(baseURL, "500x500-000000-80-0-0.jpg"),
			Size:      models.Large,
			MediaType: "image/jpg",
		},
		{
			URL:       path.Join(baseURL, "250x250-000000-80-0-0.jpg"),
			Size:      models.Medium,
			MediaType: "image/jpg",
		},
		{
			URL:       path.Join(baseURL, "56x56-000000-80-0-0.jpg"),
			Size:      models.Small,
			MediaType: "image/jpg",
		},
	}
}

func toQueueItem(t deezerModels.Track) models.PlayQueueItem {
	a := models.Artist{
		Deezer: models.Deezer{
			Id: t.Artist.ID,
		},
		Name:           t.Artist.Name,
		NameNormalized: t.Artist.Name,
		Id:             strconv.Itoa(t.Artist.ID),
		Image:          []models.Image{}, // The B&O app doesn't set this.
	}
	qi := models.PlayQueueItem{
		Track: &models.Track{
			Deezer: &models.Deezer{
				Id: t.ID,
			},
			Name:   t.Title,
			Artist: []models.Artist{a},
			Image:  getTrackImages(t.MD5Image),
			Id:     strconv.Itoa(t.ID),
		},
		Behaviour: models.Planned,
	}
	return qi
}

func doQueueTrack(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	play := c.String("play")
	switch play {
	case "now", "next", "last":
	default:
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	d := deezer.NewClient()
	t, err := d.GetTrack(c.Context, args.Get(1))
	if err != nil {
		return err
	}
	br := beoremote.NewClient(args.First())
	if play == "now" {
		// We clear the queue to match what the B&O app does.
		if err = br.BeoZone.ClearPlayQueue(c.Context); err != nil {
			return err
		}
	}
	return br.BeoZone.AddQueueItem(c.Context, toQueueItem(t), beoremote.When(play))
}

func doQueueDeezerAlbum(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	play := c.String("play")
	switch play {
	case "now", "next", "last":
	default:
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	d := deezer.NewClient()
	tracks, err := d.GetAlbumTracks(c.Context, args.Get(1))
	if err != nil {
		return err
	}
	br := beoremote.NewClient(args.First())
	if play == "now" {
		// We clear the queue to match what the B&O app does.
		if err = br.BeoZone.ClearPlayQueue(c.Context); err != nil {
			return err
		}
	}
	var items []models.PlayQueueItem
	for _, t := range tracks {
		items = append(items, toQueueItem(t))
	}
	return br.BeoZone.AddDeezerTracks(c.Context, items, beoremote.When(play))
}

func doGetTimers(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	timers, err := br.BeoHome.GetTimers(c.Context)
	if err != nil {
		return err
	}
	if len(timers) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "No timers set.")
		return nil
	}
	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 8, 4, 1, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tTIME\tRECURRING\tACTION")
	for _, t := range timers {
		recurring := ""
		for i, s := range t.Recurring {
			if i > 0 {
				recurring += ","
			}
			switch s {
			case models.Monday:
				recurring += "mon"
			case models.Tuesday:
				recurring += "tue"
			case models.Wednesday:
				recurring += "wed"
			case models.Thursday:
				recurring += "thur"
			case models.Friday:
				recurring += "fri"
			case models.Saturday:
				recurring += "sat"
			case models.Sunday:
				recurring += "sun"
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.Id,
			t.FriendlyName, t.Time, recurring, t.ActionType)
	}
	_ = tw.Flush()
	return nil
}

func doDeleteTimer(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 2 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
	return br.BeoHome.DeleteTimer(c.Context, args.Get(1))
}

func doWatchNotifications(c *cli.Context) error {
	args := c.Args()
	if args.Len() != 1 {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}
	br := beoremote.NewClient(args.First())
retry:
	events, err := br.BeoZone.OpenNotificationStream(c.Context)
	if err != nil {
		return err
	}
	for event := range events {
		// Handle the product closing the connection on us.
		// It's not clear why they do this.
		if errors.Is(event.Err, io.EOF) {
			fmt.Printf("[Reconnecting...]\n\n")
			goto retry
		}
		var n models.NotificationWrapper
		if event.Err != nil {
			fmt.Printf("Error: %v\n\n", event.Err)
		} else if err = json.Unmarshal(event.Value, &n); err != nil {
			fmt.Printf("Error: %s\n\n", err)
		} else {
			fmt.Printf("Type: %s\n", n.Notification.Type)
			fmt.Printf("Kind: %s\n", n.Notification.Kind)
			fmt.Printf("Timestamp: %s\n", n.Notification.Timestamp)
			fmt.Printf("Data: %s\n\n", string(n.Notification.Data))
		}
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	app := &cli.App{
		Name:  "beoutil",
		Usage: "Control B&O products via the beoremote API",
	}
	app.Commands = append(app.Commands, &cli.Command{
		Name:   "find-products",
		Usage:  "Discover products using MDNS",
		Action: doFindProducts,
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 5 * time.Second,
			},
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:   "list-products",
		Usage:  "List discovered products",
		Action: doListProducts,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:     "all-standby",
		Usage:    "Put all products into standby",
		Category: "Power Management",
		Action:   doAllStandby,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "standby",
		Usage:     "Put product into standby mode",
		ArgsUsage: "<product IP>",
		Category:  "Power Management",
		Action:    doStandby,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "poweron",
		Usage:     "Power on product",
		ArgsUsage: "<product IP>",
		Category:  "Power Management",
		Action:    doPowerOn,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "reboot",
		Usage:     "Reboot product",
		ArgsUsage: "<product IP>",
		Category:  "Power Management",
		Action:    doReboot,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-volume",
		Usage:     "Set speaker volume",
		ArgsUsage: "<product IP>",
		Category:  "Speaker",
		Action:    doGetVolume,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "set-volume",
		Usage:     "Get speaker volume",
		ArgsUsage: "<product IP> <0-100>",
		Category:  "Speaker",
		Action:    doSetVolume,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-muted",
		Usage:     "Set speaker volume",
		ArgsUsage: "<product IP>",
		Category:  "Speaker",
		Action:    doGetMuted,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "set-muted",
		Usage:     "Get speaker volume",
		ArgsUsage: "<product IP> <true|false>",
		Category:  "Speaker",
		Action:    doSetMuted,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "pause",
		Usage:     "Pause the stream",
		ArgsUsage: "<product IP>",
		Category:  "Stream",
		Action:    doPause,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "play",
		Usage:     "Unpause the stream",
		ArgsUsage: "<product IP>",
		Category:  "Stream",
		Action:    doPlay,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "forward",
		Usage:     "Play the next track",
		ArgsUsage: "<product IP>",
		Category:  "Stream",
		Action:    doForward,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "backward",
		Usage:     "Play the previous track",
		ArgsUsage: "<product IP>",
		Category:  "Stream",
		Action:    doBackward,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "stop",
		Usage:     "Stop playback",
		ArgsUsage: "<product IP>",
		Category:  "Stream",
		Action:    doStop,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-queue",
		Usage:     "Get play queue",
		ArgsUsage: "<product IP>",
		Category:  "Queue",
		Action:    doGetQueue,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "clear-queue",
		Usage:     "Clear play queue",
		ArgsUsage: "<product IP>",
		Category:  "Queue",
		Action:    doClearQueue,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "remove-qitem",
		Usage:     "Removed item from the play queue",
		ArgsUsage: "<product IP> <playlist ID>",
		Category:  "Queue",
		Action:    doRemoveQueueItem,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "move-qitem",
		Usage:     "Move an item in the play queue",
		ArgsUsage: "<product IP> <playlist ID> <before playlist ID>",
		Category:  "Queue",
		Action:    doMoveQueueItem,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "play-qitem",
		Usage:     "Play queue from the specified item",
		ArgsUsage: "<product IP> <playlist ID>",
		Category:  "Queue",
		Action:    doPlayQueueItem,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "set-repeat",
		Usage:     "Set queue repeat mode",
		ArgsUsage: "<product IP> <current|all|off>",
		Category:  "Queue",
		Action:    doSetRepeat,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "set-random",
		Usage:     "Set queue random mode",
		ArgsUsage: "<product IP> <on|off>",
		Category:  "Queue",
		Action:    doSetRandom,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "search-artist",
		Usage:     "Search for an artist on deezer",
		ArgsUsage: "<query string>",
		Category:  "Deezer",
		Action:    doSearchArtist,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of results",
				Value: 10,
				Base:  10,
			},
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "list-albums",
		Usage:     "List albums by an artist",
		ArgsUsage: "<artist ID>",
		Category:  "Deezer",
		Action:    doListAlbums,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "list-tracks",
		Usage:     "List tracks on an album",
		ArgsUsage: "<album ID>",
		Category:  "Deezer",
		Action:    doListTracks,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "queue-track",
		Usage:     "Queue a track from deezer",
		ArgsUsage: "<product IP> <track ID>",
		Category:  "Deezer",
		Action:    doQueueTrack,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "play",
				Value: "last",
				Usage: "(values: now,next,last)",
			},
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "queue-album",
		Usage:     "Queue an album from deezer",
		ArgsUsage: "<product IP> <album ID>",
		Category:  "Deezer",
		Action:    doQueueDeezerAlbum,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "play",
				Value: "last",
				Usage: "(values: now,next,last)",
			},
		},
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-sources",
		Usage:     "Get sources available to product",
		ArgsUsage: "<product IP>",
		Category:  "Sources",
		Action:    doGetSources,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-active",
		Usage:     "Get active source",
		ArgsUsage: "<product IP>",
		Category:  "Sources",
		Action:    doGetActiveSource,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "set-active",
		Usage:     "Set active source",
		ArgsUsage: "<product IP> <source ID>",
		Category:  "Sources",
		Action:    doSetActiveSource,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "clear-active",
		Usage:     "Clear active source",
		ArgsUsage: "<product IP>",
		Category:  "Sources",
		Action:    doClearActiveSource,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "add-listener",
		Usage:     "Add listener to primary experience",
		ArgsUsage: "<product IP> <listener JID>",
		Category:  "Multiroom",
		Action:    doAddListener,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "remove-listener",
		Usage:     "Remove listener from primary experience",
		ArgsUsage: "<product IP> <listener JID>",
		Category:  "Multiroom",
		Action:    doRemoveListener,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "get-timers",
		Usage:     "Get timers from product",
		ArgsUsage: "<product IP>",
		Category:  "Timers",
		Action:    doGetTimers,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "delete-timer",
		Usage:     "Delete a timer",
		ArgsUsage: "<product IP> <timer ID>",
		Category:  "Timers",
		Action:    doDeleteTimer,
	})
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "watch",
		Usage:     "Watch notifications from product",
		ArgsUsage: "<product IP>",
		Category:  "Notifications",
		Action:    doWatchNotifications,
	})
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
