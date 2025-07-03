package usecase

import "auth/driver/routes/request"

type SessionUseCase struct{}

func (uc SessionUseCase) Post(req request.SessionRequest) {}

func (uc SessionUseCase) Get(req request.SessionRequest) {}

func (uc SessionUseCase) Put(req request.SessionRequest) {}

func (uc SessionUseCase) Delete(req request.SessionRequest) {}
