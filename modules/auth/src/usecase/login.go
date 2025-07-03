package usecase

import "auth/driver/routes/request"

type LoginUseCase struct{}

func (uc LoginUseCase) Post(req request.LoginRequest) {}

func (uc LoginUseCase) Get(req request.LoginRequest) {}

func (uc LoginUseCase) Put(req request.LoginRequest) {}

func (uc LoginUseCase) Delete(req request.LoginRequest) {}
