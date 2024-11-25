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

package beoremote

import (
	"context"
	"fmt"
	"net/url"

	"beoutil/clients/beoremote/models"
	"beoutil/clients/rest"
)

type When string

const (
	Now  When = "now"
	Next When = "next"
)

type BeoZone interface {
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Forward(ctx context.Context) error
	Backward(ctx context.Context) error
	Stop(ctx context.Context) error
	GetVolume(ctx context.Context) (int, error)
	SetVolume(ctx context.Context, level int) error
	GetMuted(ctx context.Context) (bool, error)
	SetMuted(ctx context.Context, muted bool) error
	ToggleRepeat(ctx context.Context) error
	ToggleRandom(ctx context.Context) error
	GetPlayQueue(ctx context.Context, offset, count int) (*models.PlayQueue, error)
	ClearPlayQueue(ctx context.Context) error
	RemoveQueueItem(ctx context.Context, id string) error
	AddQueueItem(ctx context.Context, qi models.PlayQueueItem, play When) error
	AddDeezerTracks(ctx context.Context, qi []models.PlayQueueItem, play When) error
	MoveQueueItem(ctx context.Context, id, bid string) error
	PlayQueueItem(ctx context.Context, id string) error
	SetQueueRepeat(ctx context.Context, repeat models.Repeat) error
	SetQueueRandom(ctx context.Context, random models.Random) error
	GetActiveSources(ctx context.Context) (*models.ActiveSourcesResponse, error)
	PlaySource(ctx context.Context, sourceID models.SourceID) error
	AddListener(ctx context.Context, jid models.Jid) error
	RemoveListener(ctx context.Context, jid models.Jid) error
	EndExperience(ctx context.Context) error
	GetSystemProducts(ctx context.Context) ([]models.Product, error)
	OpenNotificationStream(ctx context.Context) (<-chan rest.Event, error)
}

type beoZone struct {
	client  rest.Client
	baseURL string
}

func (z *beoZone) Play(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/Stream/Play", nil)
	return err
}

func (z *beoZone) Pause(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/Stream/Pause", nil)
	return err
}

func (z *beoZone) Forward(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/Stream/Forward", nil)
	return err
}

func (z *beoZone) Backward(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/Stream/Backward", nil)
	return err
}

func (z *beoZone) Stop(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/Stream/Stop", nil)
	return err
}

func (z *beoZone) GetVolume(ctx context.Context) (int, error) {
	var r models.SpeakerLevel
	err := z.client.DoGet(ctx, z.baseURL+"/BeoZone/Zone/Sound/Volume/Speaker/Level", &r)
	if err != nil {
		return 0, err
	}
	return r.Level, nil
}

func (z *beoZone) SetVolume(ctx context.Context, level int) error {
	r := models.SpeakerLevel{
		Level: level,
	}
	_, err := z.client.DoPut(ctx, z.baseURL+"/BeoZone/Zone/Sound/Volume/Speaker/Level", r)
	return err
}

func (z *beoZone) GetMuted(ctx context.Context) (bool, error) {
	var r models.SpeakerMuted
	err := z.client.DoGet(ctx, z.baseURL+"/BeoZone/Zone/Sound/Volume/Speaker/Muted", &r)
	if err != nil {
		return false, err
	}
	return r.Muted, nil
}

func (z *beoZone) SetMuted(ctx context.Context, muted bool) error {
	r := models.SpeakerMuted{
		Muted: muted,
	}
	_, err := z.client.DoPut(ctx, z.baseURL+"/BeoZone/Zone/Sound/Volume/Speaker/Muted", r)
	return err
}

func (z *beoZone) ToggleRepeat(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/List/Repeat", nil)
	return err
}

func (z *beoZone) ToggleRandom(ctx context.Context) error {
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/List/Shuffle", nil)
	return err
}

func (z *beoZone) GetPlayQueue(ctx context.Context, offset, count int) (*models.PlayQueue, error) {
	var r models.PlayQueueResponse
	endPoint := "/BeoZone/Zone/PlayQueue/"
	if offset != 0 || count != 0 {
		endPoint += "?"
	}
	if offset != 0 {
		endPoint += fmt.Sprintf("offset=%d", offset)
	}
	if offset != 0 && count != 0 {
		endPoint += "&"
	}
	if count != 0 {
		endPoint += fmt.Sprintf("count=%d", count)
	}
	err := z.client.DoGet(ctx, z.baseURL+endPoint, &r)
	if err != nil {
		return nil, err
	}
	return &r.PlayQueue, nil
}

func (z *beoZone) setPlayQueue(ctx context.Context, q *models.PlayQueue) error {
	_, err := z.client.DoPut(ctx, z.baseURL+"/BeoZone/Zone/PlayQueue/", q)
	return err
}

func (z *beoZone) ClearPlayQueue(ctx context.Context) error {
	_, err := z.client.DoDelete(ctx, z.baseURL+"/BeoZone/Zone/PlayQueue")
	return err
}

func (z *beoZone) RemoveQueueItem(ctx context.Context, id string) error {
	endPointFmt := "/BeoZone/Zone/PlayQueue/plid-%s"
	_, err := z.client.DoDelete(ctx, z.baseURL+fmt.Sprintf(endPointFmt, id))
	return err
}

func (z *beoZone) MoveQueueItem(ctx context.Context, id, bid string) error {
	endPointFmt := "/BeoZone/Zone/PlayQueue/plid-%s?id=plid-%s"
	_, err := z.client.DoPost(ctx, z.baseURL+fmt.Sprintf(endPointFmt, id, bid), nil)
	return err
}

func (z *beoZone) PlayQueueItem(ctx context.Context, id string) error {
	r := models.PlayPointerRequest{
		PlayPointer: models.PlayPointer{
			PlayQueueItemId: "plid-" + id,
		},
	}
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/PlayQueue/PlayPointer", r)
	return err
}

func (z *beoZone) SetQueueRepeat(ctx context.Context, repeat models.Repeat) error {
	return z.setPlayQueue(ctx, &models.PlayQueue{Repeat: repeat})
}

func (z *beoZone) SetQueueRandom(ctx context.Context, random models.Random) error {
	return z.setPlayQueue(ctx, &models.PlayQueue{Random: random})
}

func (z *beoZone) AddQueueItem(ctx context.Context, qi models.PlayQueueItem, play When) error {
	endpoint := "/BeoZone/Zone/PlayQueue/"
	switch play {
	case Now:
		endpoint += "?instantplay"
	case Next:
		endpoint += "?id=&insert=after"
	}
	r := models.PlayQueueItemRequest{
		PlayQueueItem: qi,
	}
	_, err := z.client.DoPost(ctx, z.baseURL+endpoint, r)
	return err
}

func (z *beoZone) AddDeezerTracks(ctx context.Context, qi []models.PlayQueueItem, play When) error {
	endpoint := "/BeoZone/Zone/PlayQueue/"
	// Note: it's possible to insert a track of playlist in the queue
	// before another queue item by specifying the id parameter, but this
	// isn't something we expose right now.
	switch play {
	case Now:
		endpoint += "?instantplay"
	case Next:
		endpoint += "?id=&insert=after"
	}
	r := models.PlayQueue{
		PlayQueueItem: qi,
		Container: models.Container{
			Type: models.DeezerPlaylist,
			Deezer: models.Deezer{
				Id: qi[0].Track.Deezer.Id,
			},
		},
	}
	_, err := z.client.DoPost(ctx, z.baseURL+endpoint, r)
	return err
}

func (z *beoZone) GetActiveSources(ctx context.Context) (*models.ActiveSourcesResponse, error) {
	r := new(models.ActiveSourcesResponse)
	if err := z.client.DoGet(ctx, z.baseURL+"/BeoZone/Zone/ActiveSources", r); err != nil {
		return nil, err
	}
	return r, nil
}

func (z *beoZone) PlaySource(ctx context.Context, sourceID models.SourceID) error {
	r := models.ActiveSourcesRequest{
		PrimaryExperience: models.PrimaryExperience{
			Source: models.Source{
				Id: sourceID,
			},
		},
	}
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/ActiveSources", r)
	return err
}

func (z *beoZone) AddListener(ctx context.Context, jid models.Jid) error {
	r := models.PrimaryExperienceRequest{
		Listener: models.Listener{
			Jid: jid,
		},
	}
	_, err := z.client.DoPost(ctx, z.baseURL+"/BeoZone/Zone/ActiveSources/primaryExperience", r)
	return err
}

func (z *beoZone) RemoveListener(ctx context.Context, jid models.Jid) error {
	endPointFmt := "/BeoZone/Zone/ActiveSources/primaryExperience?jid=%s"
	_, err := z.client.DoDelete(ctx, z.baseURL+fmt.Sprintf(endPointFmt, url.QueryEscape(string(jid))))
	return err
}

func (z *beoZone) EndExperience(ctx context.Context) error {
	_, err := z.client.DoDelete(ctx, z.baseURL+"/BeoZone/Zone/ActiveSources/primaryExperience")
	return err
}

func (z *beoZone) GetSystemProducts(ctx context.Context) ([]models.Product, error) {
	var r models.ProductsResponse
	err := z.client.DoGet(ctx, z.baseURL+"/BeoZone/System/Products", &r)
	if err != nil {
		return nil, err
	}
	return r.Products, nil
}

func (z *beoZone) OpenNotificationStream(ctx context.Context) (<-chan rest.Event, error) {
	return z.client.OpenEventStream(ctx, z.baseURL+"/BeoNotify/Notifications?timeout=86400")
}
