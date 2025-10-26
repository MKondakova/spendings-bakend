package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/stretchr/testify/suite"
)

type SuiteWithRequests struct {
	suite.Suite
}

func (s *SuiteWithRequests) GetAPI(
	serviceHost, path string,
	headers map[string]string,
	cookies []*http.Cookie,
) ([]byte, int) {
	s.T().Helper()

	res, _, statusCode := s.DoRequest(http.MethodGet, serviceHost, path, nil, headers, cookies)

	return res, statusCode
}

func (s *SuiteWithRequests) DeleteAPI(
	serviceHost, path string,
	headers map[string]string,
	cookies []*http.Cookie,
) ([]byte, int) {
	s.T().Helper()

	res, _, statusCode := s.DoRequest(http.MethodDelete, serviceHost, path, nil, headers, cookies)

	return res, statusCode
}

func (s *SuiteWithRequests) PostAPI(
	serviceHost, path string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
) ([]byte, int) {
	s.T().Helper()

	res, _, statusCode := s.DoRequest(http.MethodPost, serviceHost, path, body, headers, cookies)

	return res, statusCode
}

func (s *SuiteWithRequests) DoRequest(
	method string,
	serviceHost string,
	path string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
) (respBody []byte, respCookies []*http.Cookie, statusCode int) {
	s.T().Helper()

	//nolint:bodyclose // body already closed in DoRequestResp
	respBody, resp := s.DoRequestResp(method, serviceHost, path, body, headers, cookies)

	return respBody, resp.Cookies(), resp.StatusCode
}

func (s *SuiteWithRequests) DoRequestResp(
	method string,
	serviceHost string,
	path string,
	body []byte,
	headers map[string]string,
	cookies []*http.Cookie,
) (respBody []byte, resp *http.Response) {
	s.T().Helper()

	buf := bytes.NewBuffer(body)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, serviceHost+path, buf)
	s.NoError(err)

	req.Header.Set("Content-Type", "application/json")

	for header, value := range headers {
		req.Header.Set(header, value)
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err = http.DefaultClient.Do(req)
	s.NoError(err)

	content, err := io.ReadAll(resp.Body)
	s.NoError(err)

	err = resp.Body.Close()
	s.NoError(err)

	return content, resp
}
