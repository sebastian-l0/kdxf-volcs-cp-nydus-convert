package errors

import stderrors "errors"

type Code string

const (
	CodeImageFileNotFound          Code = "IMAGE_FILE_NOT_FOUND"
	CodeMappingFileNotFound        Code = "MAPPING_FILE_NOT_FOUND"
	CodeInvalidImageRef            Code = "INVALID_IMAGE_REF"
	CodeTagNotFound                Code = "TAG_NOT_FOUND"
	CodeMappingNotFound            Code = "MAPPING_NOT_FOUND"
	CodeInvalidPipelineMapping     Code = "INVALID_PIPELINE_MAPPING"
	CodeInvalidConfig              Code = "INVALID_CONFIG"
	CodeRunPipelineNotImplemented  Code = "RUN_PIPELINE_NOT_IMPLEMENTED"
	CodeRunPipelineFailed          Code = "RUN_PIPELINE_FAILED"
	CodeRunPipelineResponseInvalid Code = "RUN_PIPELINE_RESPONSE_INVALID"
	CodeRateLimitWaitCanceled      Code = "RATE_LIMIT_WAIT_CANCELED"
)

type AppError struct {
	Code    Code
	Message string
	Cause   error
}

func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Cause.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func CodeOf(err error) Code {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func MessageOf(err error) string {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr.Message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
