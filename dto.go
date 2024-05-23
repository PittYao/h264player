package main

type JCodec struct {
	Type string
}

type RtspUrlDTO struct {
	RtspUrl      string `json:"rtspUrl" binding:"required"`
	DisableAudio bool   `json:"disableAudio"`
}

type ReceiverDTO struct {
	Data  string `json:"data"`
	Suuid string `json:"suuid"`
}

type ResponseDTO struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(msg string) ResponseDTO {
	var r ResponseDTO
	r.Code = 200
	r.Message = msg
	return r
}

func Error(message string) ResponseDTO {
	var r ResponseDTO
	r.Code = 500
	r.Message = message
	r.Data = ""
	return r
}

func (r *ResponseDTO) Success(msg string) *ResponseDTO {
	r.Code = 200
	r.Message = msg
	return r
}

func (r *ResponseDTO) SuccessWithData(msg string, data interface{}) *ResponseDTO {
	r.Code = 200
	r.Message = msg
	r.Data = data
	return r
}
