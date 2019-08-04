package gobac

import (
	"fmt"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
	"net"
)

type Response struct {
	Pdu
	Valid                bool
	Sender               *net.UDPAddr
	MaxSegments          uint8
	MaxAPDU              uint16
	SequenceNumber       uint32
	ProposedWindowNumber uint32
	IsBroadcast          bool

	Failed bool

	Errored          bool
	ErrorClass       types.ErrorClass
	ErrorClassString string
	ErrorCode        types.ErrorCode
	ErrorCodeString  string

	Aborted           bool
	AbortReason       types.AbortReason
	AbortReasonString string

	Rejected           bool
	RejectReason       types.RejectReason
	RejectReasonString string

	Server *Server

	Dest interface{}
}

func NewResponse(data []byte, server *Server, address net.IP) *Response {
	pdu := &Response{
		Pdu:    NewPdu(),
		Server: server,
	}

	pdu.Target = address

	pdu.Pdu.Buffer = encoding.NewBuffer(data)

	return pdu
}

func (r *Response) DecodeHeader() error {
	r.ProtocolType = r.NextOne()

	if r.ProtocolType != BACnetProtocol {
		return fmt.Errorf("expected protocol to be %x but got %x", BACnetProtocol, r.ProtocolType)
	}

	r.Function = r.NextOne()

	switch r.Function {
	case OriginalUnicastNPDU:
		break

	case OriginalBroadcastNPDU:
		r.IsBroadcast = true
		break

	default:
		return fmt.Errorf("received a function type that doesn't interest us: %x", r.Function)
	}

	r.BVLCLength = r.DecodeUnsigned16()
	r.NPDULength = r.BVLCLength - 4
	r.Truncate(int(r.NPDULength))

	return nil
}

func (r *Response) DecodeNPCI() error {
	l := r.Len()

	r.ProtocolVersion = r.NextOne()

	if r.ProtocolVersion != BACnetVersion {
		return fmt.Errorf("expected protocol version to be %d but got %d", 1, r.ProtocolVersion)
	}

	ctrl := r.NextOne()

	if r.NetworkLayerMessage = ctrl&0x80 != 0; r.NetworkLayerMessage {
		return fmt.Errorf("network layer messages aren't supported")
	}

	r.ControlOctet = ctrl

	if hasDest := ctrl & 0x20; hasDest != 0 {
		// DNET, DLEN, and DADR are present
		// We don't need this info since we're not dealing with raw packets
		// Let's just shave a few bytes off the buffer
		// DNET is 2 octets
		// DLEN is 1 octet
		// DADR is DLEN
		// +1 octet for hop count
		r.Next(2)                // dnet
		dLen := r.NextOne()      // dlen
		r.Next(int(dLen))        // dadr
		r.HopCount = r.NextOne() // hopcount
	}

	if hasSrc := ctrl & 0x08; hasSrc != 0 {
		// SNET, SLEN, SADR are present
		// Let's shave the bytes off
		// SNET = 2 octets
		// SLEN = 1 octet
		// SADR = SLEN
		r.Next(2)           // snet
		sLen := r.NextOne() // slen
		r.Next(int(sLen))   // sadr
	}

	// Expecting reply is
	r.ExpectingReply = ctrl&0x04 != 0

	// Priority is the first 2 bits
	r.Priority = ctrl & 0x03

	magicByte := r.NextOne()

	r.PduType = magicByte & 0xF0

	switch r.PduType {
	case PduTypeUnconfirmedServiceRequest:
		break

	case PduTypeSegmentAck:
		fmt.Println("Got a segment ack and I don't know how to handle this")
		break
	case PduTypeConfirmedServiceRequest:
		if segmented := magicByte & 0x8; segmented != 0 {
			r.Failed = true
			r.ErrorClassString = "internal"
			r.ErrorCodeString = "segmentation is not implemented!"
			break
		}

		r.MaxSegments = encoding.DecodeMaxSegments(magicByte)
		r.MaxAPDU = encoding.DecodeMaxAPDU(magicByte)
		r.InvokeID = r.NextOne()
		r.ServiceChoice = r.NextOne()
		break

	case PduTypeSimpleAck:
		bytes := r.Bytes()
		if len(bytes) == 2 {
			r.InvokeID = r.NextOne()
		}
		r.ServiceChoice = r.NextOne()
		break

	case PduTypeComplexAck:
		r.InvokeID = r.NextOne()

		if segmented := magicByte & 0x8; segmented != 0 {
			// Sequence number
			r.NextOne()

			// Proposed window size
			r.NextOne()

			r.Failed = true
			r.ErrorClassString = "internal"
			r.ErrorCodeString = "segmentation is not implemented!"
		}

		break

	case PduTypeError:
		r.Failed = true
		r.Errored = true
		r.InvokeID = r.NextOne()
		r.ServiceChoice = r.NextOne()

		_, v := r.DecodeTag()
		r.ErrorClass = r.DecodeUnsigned(v)
		_, v = r.DecodeTag()
		r.ErrorCode = r.DecodeUnsigned(v)

		switch r.ErrorClass {
		case types.ERROR_CLASS_DEVICE:
			r.ErrorClassString = "device"
		case types.ERROR_CLASS_OBJECT:
			r.ErrorClassString = "object"
		case types.ERROR_CLASS_PROPERTY:
			r.ErrorClassString = "property"
		case types.ERROR_CLASS_RESOURCES:
			r.ErrorClassString = "resources"
		case types.ERROR_CLASS_SECURITY:
			r.ErrorClassString = "security"
		case types.ERROR_CLASS_SERVICES:
			r.ErrorClassString = "services"
		case types.ERROR_CLASS_VT:
			r.ErrorClassString = "vt"
		case types.ERROR_CLASS_COMMUNICATION:
			r.ErrorClassString = "communication"
		default:
			r.ErrorClassString = "unknown"
		}

		switch r.ErrorCode {
		case types.ErrorCodeOther:
			r.ErrorCodeString = "ErrorCodeOther"
		case types.ErrorCodeDeviceBusy:
			r.ErrorCodeString = "ErrorCodeDeviceBusy"
		case types.ErrorCodeConfigurationInProgress:
			r.ErrorCodeString = "ErrorCodeConfigurationInProgress"
		case types.ErrorCodeOperationalProblem:
			r.ErrorCodeString = "ErrorCodeOperationalProblem"
		case types.ErrorCodeDynamicCreationNotSupported:
			r.ErrorCodeString = "ErrorCodeDynamicCreationNotSupported"
		case types.ErrorCodeNoObjectsOfSpecifiedType:
			r.ErrorCodeString = "ErrorCodeNoObjectsOfSpecifiedType"
		case types.ErrorCodeObjectDeletionNotPermitted:
			r.ErrorCodeString = "ErrorCodeObjectDeletionNotPermitted"
		case types.ErrorCodeObjectIdentifierAlreadyExists:
			r.ErrorCodeString = "ErrorCodeObjectIdentifierAlreadyExists"
		case types.ErrorCodeReadAccessDenied:
			r.ErrorCodeString = "ErrorCodeReadAccessDenied"
		case types.ErrorCodeUnknownObject:
			r.ErrorCodeString = "ErrorCodeUnknownObject"
		case types.ErrorCodeUnsupportedObjectType:
			r.ErrorCodeString = "ErrorCodeUnsupportedObjectType"
		case types.ErrorCodeCharacterSetNotSupported:
			r.ErrorCodeString = "ErrorCodeCharacterSetNotSupported"
		case types.ErrorCodeDatatypeNotSupported:
			r.ErrorCodeString = "ErrorCodeDatatypeNotSupported"
		case types.ErrorCodeInconsistentSelectionCriterion:
			r.ErrorCodeString = "ErrorCodeInconsistentSelectionCriterion"
		case types.ErrorCodeInvalidArrayIndex:
			r.ErrorCodeString = "ErrorCodeInvalidArrayIndex"
		case types.ErrorCodeInvalidDataType:
			r.ErrorCodeString = "ErrorCodeInvalidDataType"
		case types.ErrorCodeNotCovProperty:
			r.ErrorCodeString = "ErrorCodeNotCovProperty"
		case types.ErrorCodeOptionalFunctionalityNotSupported:
			r.ErrorCodeString = "ErrorCodeOptionalFunctionalityNotSupported"
		case types.ErrorCodePropertyIsNotAnArray:
			r.ErrorCodeString = "ErrorCodePropertyIsNotAnArray"
		case types.ErrorCodeUnknownProperty:
			r.ErrorCodeString = "ErrorCodeUnknownProperty"
		case types.ErrorCodeValueOutOfRange:
			r.ErrorCodeString = "ErrorCodeValueOutOfRange"
		case types.ErrorCodeWriteAccessDenied:
			r.ErrorCodeString = "ErrorCodeWriteAccessDenied"
		case types.ErrorCodeNoSpaceForObject:
			r.ErrorCodeString = "ErrorCodeNoSpaceForObject"
		case types.ErrorCodeNoSpaceToAddListElement:
			r.ErrorCodeString = "ErrorCodeNoSpaceToAddListElement"
		case types.ErrorCodeNoSpaceToWriteProperty:
			r.ErrorCodeString = "ErrorCodeNoSpaceToWriteProperty"
		case types.ErrorCodeAuthenticationFailed:
			r.ErrorCodeString = "ErrorCodeAuthenticationFailed"
		case types.ErrorCodeIncompatibleSecurityLevels:
			r.ErrorCodeString = "ErrorCodeIncompatibleSecurityLevels"
		case types.ErrorCodeInvalidOperatorName:
			r.ErrorCodeString = "ErrorCodeInvalidOperatorName"
		case types.ErrorCodeKeyGenerationError:
			r.ErrorCodeString = "ErrorCodeKeyGenerationError"
		case types.ErrorCodePasswordFailure:
			r.ErrorCodeString = "ErrorCodePasswordFailure"
		case types.ErrorCodeSecurityNotSupported:
			r.ErrorCodeString = "ErrorCodeSecurityNotSupported"
		case types.ErrorCodeTimeout:
			r.ErrorCodeString = "ErrorCodeTimeout"
		case types.ErrorCodeCovSubscriptionFailed:
			r.ErrorCodeString = "ErrorCodeCovSubscriptionFailed"
		case types.ErrorCodeDuplicateName:
			r.ErrorCodeString = "ErrorCodeDuplicateName"
		case types.ErrorCodeDuplicateObjectId:
			r.ErrorCodeString = "ErrorCodeDuplicateObjectId"
		case types.ErrorCodeFileAccessDenied:
			r.ErrorCodeString = "ErrorCodeFileAccessDenied"
		case types.ErrorCodeInconsistentParameters:
			r.ErrorCodeString = "ErrorCodeInconsistentParameters"
		case types.ErrorCodeInvalidConfigurationData:
			r.ErrorCodeString = "ErrorCodeInvalidConfigurationData"
		case types.ErrorCodeInvalidFileAccessMethod:
			r.ErrorCodeString = "ErrorCodeInvalidFileAccessMethod"
		case types.ErrorCodeInvalidFileStartPosition:
			r.ErrorCodeString = "ErrorCodeInvalidFileStartPosition"
		case types.ErrorCodeInvalidParameterDataType:
			r.ErrorCodeString = "ErrorCodeInvalidParameterDataType"
		case types.ErrorCodeInvalidTimeStamp:
			r.ErrorCodeString = "ErrorCodeInvalidTimeStamp"
		case types.ErrorCodeMissingRequiredParameter:
			r.ErrorCodeString = "ErrorCodeMissingRequiredParameter"
		case types.ErrorCodePropertyIsNotAList:
			r.ErrorCodeString = "ErrorCodePropertyIsNotAList"
		case types.ErrorCodeServiceRequestDenied:
			r.ErrorCodeString = "ErrorCodeServiceRequestDenied"
		case types.ErrorCodeUnknownVtClass:
			r.ErrorCodeString = "ErrorCodeUnknownVtClass"
		case types.ErrorCodeUnknownVtSession:
			r.ErrorCodeString = "ErrorCodeUnknownVtSession"
		case types.ErrorCodeNoVtSessionsAvailable:
			r.ErrorCodeString = "ErrorCodeNoVtSessionsAvailable"
		case types.ErrorCodeVtSessionAlreadyClosed:
			r.ErrorCodeString = "ErrorCodeVtSessionAlreadyClosed"
		case types.ErrorCodeVtSessionTerminationFailure:
			r.ErrorCodeString = "ErrorCodeVtSessionTerminationFailure"
		case types.ErrorCodeReserved1:
			r.ErrorCodeString = "ErrorCodeReserved1"
		case types.ErrorCodeAbortBufferOverflow:
			r.ErrorCodeString = "ErrorCodeAbortBufferOverflow"
		case types.ErrorCodeAbortInvalidApduInThisState:
			r.ErrorCodeString = "ErrorCodeAbortInvalidApduInThisState"
		case types.ErrorCodeAbortPreemptedByHigherPriorityTask:
			r.ErrorCodeString = "ErrorCodeAbortPreemptedByHigherPriorityTask"
		case types.ErrorCodeAbortSegmentationNotSupported:
			r.ErrorCodeString = "ErrorCodeAbortSegmentationNotSupported"
		case types.ErrorCodeAbortProprietary:
			r.ErrorCodeString = "ErrorCodeAbortProprietary"
		case types.ErrorCodeAbortOther:
			r.ErrorCodeString = "ErrorCodeAbortOther"
		case types.ErrorCodeInvalidTag:
			r.ErrorCodeString = "ErrorCodeInvalidTag"
		case types.ErrorCodeNetworkDown:
			r.ErrorCodeString = "ErrorCodeNetworkDown"
		case types.ErrorCodeRejectBufferOverflow:
			r.ErrorCodeString = "ErrorCodeRejectBufferOverflow"
		case types.ErrorCodeRejectInconsistentParameters:
			r.ErrorCodeString = "ErrorCodeRejectInconsistentParameters"
		case types.ErrorCodeRejectInvalidParameterDataType:
			r.ErrorCodeString = "ErrorCodeRejectInvalidParameterDataType"
		case types.ErrorCodeRejectInvalidTag:
			r.ErrorCodeString = "ErrorCodeRejectInvalidTag"
		case types.ErrorCodeRejectMissingRequiredParameter:
			r.ErrorCodeString = "ErrorCodeRejectMissingRequiredParameter"
		case types.ErrorCodeRejectParameterOutOfRange:
			r.ErrorCodeString = "ErrorCodeRejectParameterOutOfRange"
		case types.ErrorCodeRejectTooManyArguments:
			r.ErrorCodeString = "ErrorCodeRejectTooManyArguments"
		case types.ErrorCodeRejectUndefinedEnumeration:
			r.ErrorCodeString = "ErrorCodeRejectUndefinedEnumeration"
		case types.ErrorCodeRejectUnrecognizedService:
			r.ErrorCodeString = "ErrorCodeRejectUnrecognizedService"
		case types.ErrorCodeRejectProprietary:
			r.ErrorCodeString = "ErrorCodeRejectProprietary"
		case types.ErrorCodeRejectOther:
			r.ErrorCodeString = "ErrorCodeRejectOther"
		case types.ErrorCodeUnknownDevice:
			r.ErrorCodeString = "ErrorCodeUnknownDevice"
		case types.ErrorCodeUnknownRoute:
			r.ErrorCodeString = "ErrorCodeUnknownRoute"
		case types.ErrorCodeValueNotInitialized:
			r.ErrorCodeString = "ErrorCodeValueNotInitialized"
		case types.ErrorCodeInvalidEventState:
			r.ErrorCodeString = "ErrorCodeInvalidEventState"
		case types.ErrorCodeNoAlarmConfigured:
			r.ErrorCodeString = "ErrorCodeNoAlarmConfigured"
		case types.ErrorCodeLogBufferFull:
			r.ErrorCodeString = "ErrorCodeLogBufferFull"
		case types.ErrorCodeLoggedValuePurged:
			r.ErrorCodeString = "ErrorCodeLoggedValuePurged"
		case types.ErrorCodeNoPropertySpecified:
			r.ErrorCodeString = "ErrorCodeNoPropertySpecified"
		case types.ErrorCodeNotConfiguredForTriggeredLogging:
			r.ErrorCodeString = "ErrorCodeNotConfiguredForTriggeredLogging"
		case types.ErrorCodeUnknownSubscription:
			r.ErrorCodeString = "ErrorCodeUnknownSubscription"
		case types.ErrorCodeParameterOutOfRange:
			r.ErrorCodeString = "ErrorCodeParameterOutOfRange"
		case types.ErrorCodeListElementNotFound:
			r.ErrorCodeString = "ErrorCodeListElementNotFound"
		case types.ErrorCodeBusy:
			r.ErrorCodeString = "ErrorCodeBusy"
		case types.ErrorCodeCommunicationDisabled:
			r.ErrorCodeString = "ErrorCodeCommunicationDisabled"
		case types.ErrorCodeSuccess:
			r.ErrorCodeString = "ErrorCodeSuccess"
		case types.ErrorCodeAccessDenied:
			r.ErrorCodeString = "ErrorCodeAccessDenied"
		case types.ErrorCodeBadDestinationAddress:
			r.ErrorCodeString = "ErrorCodeBadDestinationAddress"
		case types.ErrorCodeBadDestinationDeviceId:
			r.ErrorCodeString = "ErrorCodeBadDestinationDeviceId"
		case types.ErrorCodeBadSignature:
			r.ErrorCodeString = "ErrorCodeBadSignature"
		case types.ErrorCodeBadSourceAddress:
			r.ErrorCodeString = "ErrorCodeBadSourceAddress"
		case types.ErrorCodeBadTimestamp:
			r.ErrorCodeString = "ErrorCodeBadTimestamp"
		case types.ErrorCodeCannotUseKey:
			r.ErrorCodeString = "ErrorCodeCannotUseKey"
		case types.ErrorCodeCannotVerifyMessageId:
			r.ErrorCodeString = "ErrorCodeCannotVerifyMessageId"
		case types.ErrorCodeCorrectKeyRevision:
			r.ErrorCodeString = "ErrorCodeCorrectKeyRevision"
		case types.ErrorCodeDestinationDeviceIdRequired:
			r.ErrorCodeString = "ErrorCodeDestinationDeviceIdRequired"
		case types.ErrorCodeDuplicateMessage:
			r.ErrorCodeString = "ErrorCodeDuplicateMessage"
		case types.ErrorCodeEncryptionNotConfigured:
			r.ErrorCodeString = "ErrorCodeEncryptionNotConfigured"
		case types.ErrorCodeEncryptionRequired:
			r.ErrorCodeString = "ErrorCodeEncryptionRequired"
		case types.ErrorCodeIncorrectKey:
			r.ErrorCodeString = "ErrorCodeIncorrectKey"
		case types.ErrorCodeInvalidKeyData:
			r.ErrorCodeString = "ErrorCodeInvalidKeyData"
		case types.ErrorCodeKeyUpdateInProgress:
			r.ErrorCodeString = "ErrorCodeKeyUpdateInProgress"
		case types.ErrorCodeMalformedMessage:
			r.ErrorCodeString = "ErrorCodeMalformedMessage"
		case types.ErrorCodeNotKeyServer:
			r.ErrorCodeString = "ErrorCodeNotKeyServer"
		case types.ErrorCodeSecurityNotConfigured:
			r.ErrorCodeString = "ErrorCodeSecurityNotConfigured"
		case types.ErrorCodeSourceSecurityRequired:
			r.ErrorCodeString = "ErrorCodeSourceSecurityRequired"
		case types.ErrorCodeTooManyKeys:
			r.ErrorCodeString = "ErrorCodeTooManyKeys"
		case types.ErrorCodeUnknownAuthenticationType:
			r.ErrorCodeString = "ErrorCodeUnknownAuthenticationType"
		case types.ErrorCodeUnknownKey:
			r.ErrorCodeString = "ErrorCodeUnknownKey"
		case types.ErrorCodeUnknownKeyRevision:
			r.ErrorCodeString = "ErrorCodeUnknownKeyRevision"
		case types.ErrorCodeUnknownSourceMessage:
			r.ErrorCodeString = "ErrorCodeUnknownSourceMessage"
		case types.ErrorCodeNotRouterToDnet:
			r.ErrorCodeString = "ErrorCodeNotRouterToDnet"
		case types.ErrorCodeRouterBusy:
			r.ErrorCodeString = "ErrorCodeRouterBusy"
		case types.ErrorCodeUnknownNetworkMessage:
			r.ErrorCodeString = "ErrorCodeUnknownNetworkMessage"
		case types.ErrorCodeMessageTooLong:
			r.ErrorCodeString = "ErrorCodeMessageTooLong"
		case types.ErrorCodeSecurityError:
			r.ErrorCodeString = "ErrorCodeSecurityError"
		case types.ErrorCodeAddressingError:
			r.ErrorCodeString = "ErrorCodeAddressingError"
		case types.ErrorCodeWriteBdtFailed:
			r.ErrorCodeString = "ErrorCodeWriteBdtFailed"
		case types.ErrorCodeReadBdtFailed:
			r.ErrorCodeString = "ErrorCodeReadBdtFailed"
		case types.ErrorCodeRegisterForeignDeviceFailed:
			r.ErrorCodeString = "ErrorCodeRegisterForeignDeviceFailed"
		case types.ErrorCodeReadFdtFailed:
			r.ErrorCodeString = "ErrorCodeReadFdtFailed"
		case types.ErrorCodeDeleteFdtEntryFailed:
			r.ErrorCodeString = "ErrorCodeDeleteFdtEntryFailed"
		case types.ErrorCodeDistributeBroadcastFailed:
			r.ErrorCodeString = "ErrorCodeDistributeBroadcastFailed"
		case types.ErrorCodeUnknownFileSize:
			r.ErrorCodeString = "ErrorCodeUnknownFileSize"
		case types.ErrorCodeAbortApduTooLong:
			r.ErrorCodeString = "ErrorCodeAbortApduTooLong"
		case types.ErrorCodeAbortApplicationExceededReplyTime:
			r.ErrorCodeString = "ErrorCodeAbortApplicationExceededReplyTime"
		case types.ErrorCodeAbortOutOfResources:
			r.ErrorCodeString = "ErrorCodeAbortOutOfResources"
		case types.ErrorCodeAbortTsmTimeout:
			r.ErrorCodeString = "ErrorCodeAbortTsmTimeout"
		case types.ErrorCodeAbortWindowSizeOutOfRange:
			r.ErrorCodeString = "ErrorCodeAbortWindowSizeOutOfRange"
		case types.ErrorCodeFileFull:
			r.ErrorCodeString = "ErrorCodeFileFull"
		case types.ErrorCodeInconsistentConfiguration:
			r.ErrorCodeString = "ErrorCodeInconsistentConfiguration"
		case types.ErrorCodeInconsistentObjectType:
			r.ErrorCodeString = "ErrorCodeInconsistentObjectType"
		case types.ErrorCodeInternalError:
			r.ErrorCodeString = "ErrorCodeInternalError"
		case types.ErrorCodeNotConfigured:
			r.ErrorCodeString = "ErrorCodeNotConfigured"
		case types.ErrorCodeOutOfMemory:
			r.ErrorCodeString = "ErrorCodeOutOfMemory"
		case types.ErrorCodeValueTooLong:
			r.ErrorCodeString = "ErrorCodeValueTooLong"
		case types.ErrorCodeAbortInsufficientSecurity:
			r.ErrorCodeString = "ErrorCodeAbortInsufficientSecurity"
		case types.ErrorCodeAbortSecurityError:
			r.ErrorCodeString = "ErrorCodeAbortSecurityError"
		default:
			r.ErrorCodeString = "Unknown error code"
		}

		break

	case PduTypeReject:
		r.Failed = true
		r.InvokeID = r.NextOne()
		r.Rejected = true
		r.RejectReason = types.RejectReason(r.NextOne())

		switch r.RejectReason {
		case types.REJECT_REASON_BUFFER_OVERFLOW:
			r.RejectReasonString = "buffer overflow"
		case types.REJECT_REASON_INCONSISTENT_PARAMETERS:
			r.RejectReasonString = "inconsistent parameters"
		case types.REJECT_REASON_INVALID_PARAMETER_DATA_TYPE:
			r.RejectReasonString = "invalid parameter data type"
		case types.REJECT_REASON_INVALID_TAG:
			r.RejectReasonString = "invalid tag"
		case types.REJECT_REASON_MISSING_REQUIRED_PARAMETER:
			r.RejectReasonString = "missing required parameter"
		case types.REJECT_REASON_PARAMETER_OUT_OF_RANGE:
			r.RejectReasonString = "parameter out of range"
		case types.REJECT_REASON_TOO_MANY_ARGUMENTS:
			r.RejectReasonString = "too many arguments"
		case types.REJECT_REASON_UNDEFINED_ENUMERATION:
			r.RejectReasonString = "undefined enumeration"
		case types.REJECT_REASON_UNRECOGNIZED_SERVICE:
			r.RejectReasonString = "unrecognized service"
		default:
			r.RejectReasonString = "unknown reason"
		}
		break

	case PduTypeAbort:
		r.Failed = true
		_ = r.NextOne() // Server
		r.InvokeID = r.NextOne() & 0x01
		r.Aborted = true
		r.AbortReason = types.AbortReason(r.NextOne())

		switch r.AbortReason {
		case types.ABORT_REASON_BUFFER_OVERFLOW:
			r.AbortReasonString = "buffer overflow"
		case types.ABORT_REASON_INVALID_APDU_IN_THIS_STATE:
			r.AbortReasonString = "invalid APDU in this state"
		case types.ABORT_REASON_PREEMPTED_BY_HIGHER_PRIORITY_TASK:
			r.AbortReasonString = "preempted by higher priority task"
		case types.ABORT_REASON_SEGMENTATION_NOT_SUPPORTED:
			r.AbortReasonString = "segmentation not supported"
		case types.ABORT_REASON_SECURITY_ERROR:
			r.AbortReasonString = "security error"
		case types.ABORT_REASON_INSUFFICIENT_SECURITY:
			r.AbortReasonString = "insufficient security"
		default:
			r.AbortReasonString = "unknown reason"
		}
		break

	default:
		return fmt.Errorf("unsupported pdu type: %x", r.PduType)
	}

	r.ServiceChoice = r.NextOne()
	r.MessageLength = r.NPDULength - uint16(l-r.Len())

	return nil
}

func (r *Response) Decode() error {
	if err := r.DecodeHeader(); err != nil {
		r.Valid = false
		return err
	}

	if err := r.DecodeNPCI(); err != nil {
		r.Valid = false
		return err
	}

	r.Valid = true

	if r.InvokeID > 0 {
		switch r.ServiceChoice {
		case ConfirmedServiceCovNotification:
			return r.DecodeCovNotification()

		case ConfirmedServiceReadProperty:
			return r.DecodeReadPropertyApdu()
		}
	} else {
		switch r.ServiceChoice {
		case UnconfirmedServiceIAm:
			return r.DecodeIAmApdu()
		}
	}

	return nil
}
