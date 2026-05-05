package service

import "auth/driver/routes/request"

type SessionService struct{}

func (s SessionService) Post(req request.SessionRequest) {}

func (s SessionService) Get(req request.SessionRequest) {}

func (s SessionService) Put(req request.SessionRequest) {}

func (s SessionService) Delete(req request.SessionRequest) {}
