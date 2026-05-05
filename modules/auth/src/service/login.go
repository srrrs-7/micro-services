package service

import "auth/driver/routes/request"

type LoginService struct{}

func (s LoginService) Post(req request.LoginRequest) {}

func (s LoginService) Get(req request.LoginRequest) {}

func (s LoginService) Put(req request.LoginRequest) {}

func (s LoginService) Delete(req request.LoginRequest) {}
