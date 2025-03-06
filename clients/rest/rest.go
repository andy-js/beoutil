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

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"beoutil/clients/beoremote/models"
)

type Client interface {
	DoGet(ctx context.Context, endPoint string, v interface{}) error
	DoPost(ctx context.Context, endPoint string, v interface{}) ([]byte, error)
	DoPut(ctx context.Context, endPoint string, v interface{}) ([]byte, error)
	DoDelete(ctx context.Context, endPoint string) ([]byte, error)
	OpenStream(ctx context.Context, endPoint string) (*http.Response, error)
}

type jsonClient struct {
	client *http.Client
}

func NewJSONClient() Client {
	return &jsonClient{client: new(http.Client)}
}

type HttpError struct {
	StatusCode int
	Status     string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Status)
}

func newHTTPError(resp *http.Response) error {
	return &HttpError{StatusCode: resp.StatusCode, Status: resp.Status}
}

func (c *jsonClient) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	var res []byte
	res, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if len(res) > 0 {
			// FIXME: This is beoremote specific.
			var errResponse models.ErrorResponse
			if json.Unmarshal(res, &errResponse) == nil {
				return nil, &errResponse.Error
			}
		}
		return nil, newHTTPError(resp)
	}
	return res, nil
}

func (c *jsonClient) DoGet(ctx context.Context, endPoint string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endPoint, nil)
	if err != nil {
		return err
	}
	var b []byte
	b, err = c.doRequest(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func jsonEncode(v interface{}) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b), nil
}

func (c *jsonClient) DoPost(ctx context.Context, endPoint string, v interface{}) ([]byte, error) {
	var (
		r   io.Reader
		req *http.Request
		err error
	)
	if v != nil {
		r, err = jsonEncode(v)
		if err != nil {
			return nil, err
		}
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endPoint, r)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *jsonClient) DoPut(ctx context.Context, endPoint string, v interface{}) ([]byte, error) {
	r, err := jsonEncode(v)
	if err != nil {
		return nil, err
	}
	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, endPoint, r)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *jsonClient) DoDelete(ctx context.Context, endPoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endPoint, nil)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *jsonClient) OpenStream(ctx context.Context, endPoint string) (*http.Response, error) {
	var (
		req  *http.Request
		resp *http.Response
		err  error
	)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, endPoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err = c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_ = resp.Body.Close()
		return nil, newHTTPError(resp)
	}
	return resp, nil
}
