package types

import "errors"

const (
	ErrorCodeOther                              ErrorCode = 0
	ErrorCodeDeviceBusy                                   = 3
	ErrorCodeConfigurationInProgress                      = 2
	ErrorCodeOperationalProblem                           = 25
	ErrorCodeDynamicCreationNotSupported                  = 4
	ErrorCodeNoObjectsOfSpecifiedType                     = 17
	ErrorCodeObjectDeletionNotPermitted                   = 23
	ErrorCodeObjectIdentifierAlreadyExists                = 24
	ErrorCodeReadAccessDenied                             = 27
	ErrorCodeUnknownObject                                = 31
	ErrorCodeUnsupportedObjectType                        = 36
	ErrorCodeCharacterSetNotSupported                     = 41
	ErrorCodeDatatypeNotSupported                         = 47
	ErrorCodeInconsistentSelectionCriterion               = 8
	ErrorCodeInvalidArrayIndex                            = 42
	ErrorCodeInvalidDataType                              = 9
	ErrorCodeNotCovProperty                               = 44
	ErrorCodeOptionalFunctionalityNotSupported            = 45
	ErrorCodePropertyIsNotAnArray                         = 50
	ErrorCodeUnknownProperty                              = 32
	ErrorCodeValueOutOfRange                              = 37
	ErrorCodeWriteAccessDenied                            = 40
	ErrorCodeNoSpaceForObject                             = 18
	ErrorCodeNoSpaceToAddListElement                      = 19
	ErrorCodeNoSpaceToWriteProperty                       = 20
	ErrorCodeAuthenticationFailed                         = 1
	ErrorCodeIncompatibleSecurityLevels                   = 6
	ErrorCodeInvalidOperatorName                          = 12
	ErrorCodeKeyGenerationError                           = 15
	ErrorCodePasswordFailure                              = 26
	ErrorCodeSecurityNotSupported                         = 28
	ErrorCodeTimeout                                      = 30
	ErrorCodeCovSubscriptionFailed                        = 43
	ErrorCodeDuplicateName                                = 48
	ErrorCodeDuplicateObjectId                            = 49
	ErrorCodeFileAccessDenied                             = 5
	ErrorCodeInconsistentParameters                       = 7
	ErrorCodeInvalidConfigurationData                     = 46
	ErrorCodeInvalidFileAccessMethod                      = 10
	ErrorCodeInvalidFileStartPosition                     = 11
	ErrorCodeInvalidParameterDataType                     = 13
	ErrorCodeInvalidTimeStamp                             = 14
	ErrorCodeMissingRequiredParameter                     = 16
	ErrorCodePropertyIsNotAList                           = 22
	ErrorCodeServiceRequestDenied                         = 29
	ErrorCodeUnknownVtClass                               = 34
	ErrorCodeUnknownVtSession                             = 35
	ErrorCodeNoVtSessionsAvailable                        = 21
	ErrorCodeVtSessionAlreadyClosed                       = 38
	ErrorCodeVtSessionTerminationFailure                  = 39
	ErrorCodeReserved1                                    = 33
	ErrorCodeAbortBufferOverflow                          = 51
	ErrorCodeAbortInvalidApduInThisState                  = 52
	ErrorCodeAbortPreemptedByHigherPriorityTask           = 53
	ErrorCodeAbortSegmentationNotSupported                = 54
	ErrorCodeAbortProprietary                             = 55
	ErrorCodeAbortOther                                   = 56
	ErrorCodeInvalidTag                                   = 57
	ErrorCodeNetworkDown                                  = 58
	ErrorCodeRejectBufferOverflow                         = 59
	ErrorCodeRejectInconsistentParameters                 = 60
	ErrorCodeRejectInvalidParameterDataType               = 61
	ErrorCodeRejectInvalidTag                             = 62
	ErrorCodeRejectMissingRequiredParameter               = 63
	ErrorCodeRejectParameterOutOfRange                    = 64
	ErrorCodeRejectTooManyArguments                       = 65
	ErrorCodeRejectUndefinedEnumeration                   = 66
	ErrorCodeRejectUnrecognizedService                    = 67
	ErrorCodeRejectProprietary                            = 68
	ErrorCodeRejectOther                                  = 69
	ErrorCodeUnknownDevice                                = 70
	ErrorCodeUnknownRoute                                 = 71
	ErrorCodeValueNotInitialized                          = 72
	ErrorCodeInvalidEventState                            = 73
	ErrorCodeNoAlarmConfigured                            = 74
	ErrorCodeLogBufferFull                                = 75
	ErrorCodeLoggedValuePurged                            = 76
	ErrorCodeNoPropertySpecified                          = 77
	ErrorCodeNotConfiguredForTriggeredLogging             = 78
	ErrorCodeUnknownSubscription                          = 79
	ErrorCodeParameterOutOfRange                          = 80
	ErrorCodeListElementNotFound                          = 81
	ErrorCodeBusy                                         = 82
	ErrorCodeCommunicationDisabled                        = 83
	ErrorCodeSuccess                                      = 84
	ErrorCodeAccessDenied                                 = 85
	ErrorCodeBadDestinationAddress                        = 86
	ErrorCodeBadDestinationDeviceId                       = 87
	ErrorCodeBadSignature                                 = 88
	ErrorCodeBadSourceAddress                             = 89
	ErrorCodeBadTimestamp                                 = 90
	ErrorCodeCannotUseKey                                 = 91
	ErrorCodeCannotVerifyMessageId                        = 92
	ErrorCodeCorrectKeyRevision                           = 93
	ErrorCodeDestinationDeviceIdRequired                  = 94
	ErrorCodeDuplicateMessage                             = 95
	ErrorCodeEncryptionNotConfigured                      = 96
	ErrorCodeEncryptionRequired                           = 97
	ErrorCodeIncorrectKey                                 = 98
	ErrorCodeInvalidKeyData                               = 99
	ErrorCodeKeyUpdateInProgress                          = 100
	ErrorCodeMalformedMessage                             = 101
	ErrorCodeNotKeyServer                                 = 102
	ErrorCodeSecurityNotConfigured                        = 103
	ErrorCodeSourceSecurityRequired                       = 104
	ErrorCodeTooManyKeys                                  = 105
	ErrorCodeUnknownAuthenticationType                    = 106
	ErrorCodeUnknownKey                                   = 107
	ErrorCodeUnknownKeyRevision                           = 108
	ErrorCodeUnknownSourceMessage                         = 109
	ErrorCodeNotRouterToDnet                              = 110
	ErrorCodeRouterBusy                                   = 111
	ErrorCodeUnknownNetworkMessage                        = 112
	ErrorCodeMessageTooLong                               = 113
	ErrorCodeSecurityError                                = 114
	ErrorCodeAddressingError                              = 115
	ErrorCodeWriteBdtFailed                               = 116
	ErrorCodeReadBdtFailed                                = 117
	ErrorCodeRegisterForeignDeviceFailed                  = 118
	ErrorCodeReadFdtFailed                                = 119
	ErrorCodeDeleteFdtEntryFailed                         = 120
	ErrorCodeDistributeBroadcastFailed                    = 121
	ErrorCodeUnknownFileSize                              = 122
	ErrorCodeAbortApduTooLong                             = 123
	ErrorCodeAbortApplicationExceededReplyTime            = 124
	ErrorCodeAbortOutOfResources                          = 125
	ErrorCodeAbortTsmTimeout                              = 126
	ErrorCodeAbortWindowSizeOutOfRange                    = 127
	ErrorCodeFileFull                                     = 128
	ErrorCodeInconsistentConfiguration                    = 129
	ErrorCodeInconsistentObjectType                       = 130
	ErrorCodeInternalError                                = 131
	ErrorCodeNotConfigured                                = 132
	ErrorCodeOutOfMemory                                  = 133
	ErrorCodeValueTooLong                                 = 134
	ErrorCodeAbortInsufficientSecurity                    = 135
	ErrorCodeAbortSecurityError                           = 136
	ErrorCodeMax                                          = 137
	ErrorCodeProprietaryMin                               = 256
	ErrorCodeProprietaryMax                               = 65535
)

type ErrorCode uint32

func (c ErrorCode) MarshalBinary() ([]byte, error) {
	return EncodeVarUint(uint32(c)), nil
}

func (c *ErrorCode) UnmarshalBinary(b []byte) error {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	*c = ErrorCode(DecodeVarUint(b))
	return nil
}

func (c ErrorCode) String() string {
	switch c {
	case ErrorCodeOther:
		return "ErrorCodeOther"
	case ErrorCodeDeviceBusy:
		return "ErrorCodeDeviceBusy"
	case ErrorCodeConfigurationInProgress:
		return "ErrorCodeConfigurationInProgress"
	case ErrorCodeOperationalProblem:
		return "ErrorCodeOperationalProblem"
	case ErrorCodeDynamicCreationNotSupported:
		return "ErrorCodeDynamicCreationNotSupported"
	case ErrorCodeNoObjectsOfSpecifiedType:
		return "ErrorCodeNoObjectsOfSpecifiedType"
	case ErrorCodeObjectDeletionNotPermitted:
		return "ErrorCodeObjectDeletionNotPermitted"
	case ErrorCodeObjectIdentifierAlreadyExists:
		return "ErrorCodeObjectIdentifierAlreadyExists"
	case ErrorCodeReadAccessDenied:
		return "ErrorCodeReadAccessDenied"
	case ErrorCodeUnknownObject:
		return "ErrorCodeUnknownObject"
	case ErrorCodeUnsupportedObjectType:
		return "ErrorCodeUnsupportedObjectType"
	case ErrorCodeCharacterSetNotSupported:
		return "ErrorCodeCharacterSetNotSupported"
	case ErrorCodeDatatypeNotSupported:
		return "ErrorCodeDatatypeNotSupported"
	case ErrorCodeInconsistentSelectionCriterion:
		return "ErrorCodeInconsistentSelectionCriterion"
	case ErrorCodeInvalidArrayIndex:
		return "ErrorCodeInvalidArrayIndex"
	case ErrorCodeInvalidDataType:
		return "ErrorCodeInvalidDataType"
	case ErrorCodeNotCovProperty:
		return "ErrorCodeNotCovProperty"
	case ErrorCodeOptionalFunctionalityNotSupported:
		return "ErrorCodeOptionalFunctionalityNotSupported"
	case ErrorCodePropertyIsNotAnArray:
		return "ErrorCodePropertyIsNotAnArray"
	case ErrorCodeUnknownProperty:
		return "ErrorCodeUnknownProperty"
	case ErrorCodeValueOutOfRange:
		return "ErrorCodeValueOutOfRange"
	case ErrorCodeWriteAccessDenied:
		return "ErrorCodeWriteAccessDenied"
	case ErrorCodeNoSpaceForObject:
		return "ErrorCodeNoSpaceForObject"
	case ErrorCodeNoSpaceToAddListElement:
		return "ErrorCodeNoSpaceToAddListElement"
	case ErrorCodeNoSpaceToWriteProperty:
		return "ErrorCodeNoSpaceToWriteProperty"
	case ErrorCodeAuthenticationFailed:
		return "ErrorCodeAuthenticationFailed"
	case ErrorCodeIncompatibleSecurityLevels:
		return "ErrorCodeIncompatibleSecurityLevels"
	case ErrorCodeInvalidOperatorName:
		return "ErrorCodeInvalidOperatorName"
	case ErrorCodeKeyGenerationError:
		return "ErrorCodeKeyGenerationError"
	case ErrorCodePasswordFailure:
		return "ErrorCodePasswordFailure"
	case ErrorCodeSecurityNotSupported:
		return "ErrorCodeSecurityNotSupported"
	case ErrorCodeTimeout:
		return "ErrorCodeTimeout"
	case ErrorCodeCovSubscriptionFailed:
		return "ErrorCodeCovSubscriptionFailed"
	case ErrorCodeDuplicateName:
		return "ErrorCodeDuplicateName"
	case ErrorCodeDuplicateObjectId:
		return "ErrorCodeDuplicateObjectId"
	case ErrorCodeFileAccessDenied:
		return "ErrorCodeFileAccessDenied"
	case ErrorCodeInconsistentParameters:
		return "ErrorCodeInconsistentParameters"
	case ErrorCodeInvalidConfigurationData:
		return "ErrorCodeInvalidConfigurationData"
	case ErrorCodeInvalidFileAccessMethod:
		return "ErrorCodeInvalidFileAccessMethod"
	case ErrorCodeInvalidFileStartPosition:
		return "ErrorCodeInvalidFileStartPosition"
	case ErrorCodeInvalidParameterDataType:
		return "ErrorCodeInvalidParameterDataType"
	case ErrorCodeInvalidTimeStamp:
		return "ErrorCodeInvalidTimeStamp"
	case ErrorCodeMissingRequiredParameter:
		return "ErrorCodeMissingRequiredParameter"
	case ErrorCodePropertyIsNotAList:
		return "ErrorCodePropertyIsNotAList"
	case ErrorCodeServiceRequestDenied:
		return "ErrorCodeServiceRequestDenied"
	case ErrorCodeUnknownVtClass:
		return "ErrorCodeUnknownVtClass"
	case ErrorCodeUnknownVtSession:
		return "ErrorCodeUnknownVtSession"
	case ErrorCodeNoVtSessionsAvailable:
		return "ErrorCodeNoVtSessionsAvailable"
	case ErrorCodeVtSessionAlreadyClosed:
		return "ErrorCodeVtSessionAlreadyClosed"
	case ErrorCodeVtSessionTerminationFailure:
		return "ErrorCodeVtSessionTerminationFailure"
	case ErrorCodeReserved1:
		return "ErrorCodeReserved1"
	case ErrorCodeAbortBufferOverflow:
		return "ErrorCodeAbortBufferOverflow"
	case ErrorCodeAbortInvalidApduInThisState:
		return "ErrorCodeAbortInvalidApduInThisState"
	case ErrorCodeAbortPreemptedByHigherPriorityTask:
		return "ErrorCodeAbortPreemptedByHigherPriorityTask"
	case ErrorCodeAbortSegmentationNotSupported:
		return "ErrorCodeAbortSegmentationNotSupported"
	case ErrorCodeAbortProprietary:
		return "ErrorCodeAbortProprietary"
	case ErrorCodeAbortOther:
		return "ErrorCodeAbortOther"
	case ErrorCodeInvalidTag:
		return "ErrorCodeInvalidTag"
	case ErrorCodeNetworkDown:
		return "ErrorCodeNetworkDown"
	case ErrorCodeRejectBufferOverflow:
		return "ErrorCodeRejectBufferOverflow"
	case ErrorCodeRejectInconsistentParameters:
		return "ErrorCodeRejectInconsistentParameters"
	case ErrorCodeRejectInvalidParameterDataType:
		return "ErrorCodeRejectInvalidParameterDataType"
	case ErrorCodeRejectInvalidTag:
		return "ErrorCodeRejectInvalidTag"
	case ErrorCodeRejectMissingRequiredParameter:
		return "ErrorCodeRejectMissingRequiredParameter"
	case ErrorCodeRejectParameterOutOfRange:
		return "ErrorCodeRejectParameterOutOfRange"
	case ErrorCodeRejectTooManyArguments:
		return "ErrorCodeRejectTooManyArguments"
	case ErrorCodeRejectUndefinedEnumeration:
		return "ErrorCodeRejectUndefinedEnumeration"
	case ErrorCodeRejectUnrecognizedService:
		return "ErrorCodeRejectUnrecognizedService"
	case ErrorCodeRejectProprietary:
		return "ErrorCodeRejectProprietary"
	case ErrorCodeRejectOther:
		return "ErrorCodeRejectOther"
	case ErrorCodeUnknownDevice:
		return "ErrorCodeUnknownDevice"
	case ErrorCodeUnknownRoute:
		return "ErrorCodeUnknownRoute"
	case ErrorCodeValueNotInitialized:
		return "ErrorCodeValueNotInitialized"
	case ErrorCodeInvalidEventState:
		return "ErrorCodeInvalidEventState"
	case ErrorCodeNoAlarmConfigured:
		return "ErrorCodeNoAlarmConfigured"
	case ErrorCodeLogBufferFull:
		return "ErrorCodeLogBufferFull"
	case ErrorCodeLoggedValuePurged:
		return "ErrorCodeLoggedValuePurged"
	case ErrorCodeNoPropertySpecified:
		return "ErrorCodeNoPropertySpecified"
	case ErrorCodeNotConfiguredForTriggeredLogging:
		return "ErrorCodeNotConfiguredForTriggeredLogging"
	case ErrorCodeUnknownSubscription:
		return "ErrorCodeUnknownSubscription"
	case ErrorCodeParameterOutOfRange:
		return "ErrorCodeParameterOutOfRange"
	case ErrorCodeListElementNotFound:
		return "ErrorCodeListElementNotFound"
	case ErrorCodeBusy:
		return "ErrorCodeBusy"
	case ErrorCodeCommunicationDisabled:
		return "ErrorCodeCommunicationDisabled"
	case ErrorCodeSuccess:
		return "ErrorCodeSuccess"
	case ErrorCodeAccessDenied:
		return "ErrorCodeAccessDenied"
	case ErrorCodeBadDestinationAddress:
		return "ErrorCodeBadDestinationAddress"
	case ErrorCodeBadDestinationDeviceId:
		return "ErrorCodeBadDestinationDeviceId"
	case ErrorCodeBadSignature:
		return "ErrorCodeBadSignature"
	case ErrorCodeBadSourceAddress:
		return "ErrorCodeBadSourceAddress"
	case ErrorCodeBadTimestamp:
		return "ErrorCodeBadTimestamp"
	case ErrorCodeCannotUseKey:
		return "ErrorCodeCannotUseKey"
	case ErrorCodeCannotVerifyMessageId:
		return "ErrorCodeCannotVerifyMessageId"
	case ErrorCodeCorrectKeyRevision:
		return "ErrorCodeCorrectKeyRevision"
	case ErrorCodeDestinationDeviceIdRequired:
		return "ErrorCodeDestinationDeviceIdRequired"
	case ErrorCodeDuplicateMessage:
		return "ErrorCodeDuplicateMessage"
	case ErrorCodeEncryptionNotConfigured:
		return "ErrorCodeEncryptionNotConfigured"
	case ErrorCodeEncryptionRequired:
		return "ErrorCodeEncryptionRequired"
	case ErrorCodeIncorrectKey:
		return "ErrorCodeIncorrectKey"
	case ErrorCodeInvalidKeyData:
		return "ErrorCodeInvalidKeyData"
	case ErrorCodeKeyUpdateInProgress:
		return "ErrorCodeKeyUpdateInProgress"
	case ErrorCodeMalformedMessage:
		return "ErrorCodeMalformedMessage"
	case ErrorCodeNotKeyServer:
		return "ErrorCodeNotKeyServer"
	case ErrorCodeSecurityNotConfigured:
		return "ErrorCodeSecurityNotConfigured"
	case ErrorCodeSourceSecurityRequired:
		return "ErrorCodeSourceSecurityRequired"
	case ErrorCodeTooManyKeys:
		return "ErrorCodeTooManyKeys"
	case ErrorCodeUnknownAuthenticationType:
		return "ErrorCodeUnknownAuthenticationType"
	case ErrorCodeUnknownKey:
		return "ErrorCodeUnknownKey"
	case ErrorCodeUnknownKeyRevision:
		return "ErrorCodeUnknownKeyRevision"
	case ErrorCodeUnknownSourceMessage:
		return "ErrorCodeUnknownSourceMessage"
	case ErrorCodeNotRouterToDnet:
		return "ErrorCodeNotRouterToDnet"
	case ErrorCodeRouterBusy:
		return "ErrorCodeRouterBusy"
	case ErrorCodeUnknownNetworkMessage:
		return "ErrorCodeUnknownNetworkMessage"
	case ErrorCodeMessageTooLong:
		return "ErrorCodeMessageTooLong"
	case ErrorCodeSecurityError:
		return "ErrorCodeSecurityError"
	case ErrorCodeAddressingError:
		return "ErrorCodeAddressingError"
	case ErrorCodeWriteBdtFailed:
		return "ErrorCodeWriteBdtFailed"
	case ErrorCodeReadBdtFailed:
		return "ErrorCodeReadBdtFailed"
	case ErrorCodeRegisterForeignDeviceFailed:
		return "ErrorCodeRegisterForeignDeviceFailed"
	case ErrorCodeReadFdtFailed:
		return "ErrorCodeReadFdtFailed"
	case ErrorCodeDeleteFdtEntryFailed:
		return "ErrorCodeDeleteFdtEntryFailed"
	case ErrorCodeDistributeBroadcastFailed:
		return "ErrorCodeDistributeBroadcastFailed"
	case ErrorCodeUnknownFileSize:
		return "ErrorCodeUnknownFileSize"
	case ErrorCodeAbortApduTooLong:
		return "ErrorCodeAbortApduTooLong"
	case ErrorCodeAbortApplicationExceededReplyTime:
		return "ErrorCodeAbortApplicationExceededReplyTime"
	case ErrorCodeAbortOutOfResources:
		return "ErrorCodeAbortOutOfResources"
	case ErrorCodeAbortTsmTimeout:
		return "ErrorCodeAbortTsmTimeout"
	case ErrorCodeAbortWindowSizeOutOfRange:
		return "ErrorCodeAbortWindowSizeOutOfRange"
	case ErrorCodeFileFull:
		return "ErrorCodeFileFull"
	case ErrorCodeInconsistentConfiguration:
		return "ErrorCodeInconsistentConfiguration"
	case ErrorCodeInconsistentObjectType:
		return "ErrorCodeInconsistentObjectType"
	case ErrorCodeInternalError:
		return "ErrorCodeInternalError"
	case ErrorCodeNotConfigured:
		return "ErrorCodeNotConfigured"
	case ErrorCodeOutOfMemory:
		return "ErrorCodeOutOfMemory"
	case ErrorCodeValueTooLong:
		return "ErrorCodeValueTooLong"
	case ErrorCodeAbortInsufficientSecurity:
		return "ErrorCodeAbortInsufficientSecurity"
	case ErrorCodeAbortSecurityError:
		return "ErrorCodeAbortSecurityError"
	default:
		return "Unknown error code"
	}
}
