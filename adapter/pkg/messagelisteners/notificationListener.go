/*
 *  Copyright (c) 2021, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Package messagelisteners holds the implementation for event listeners functions
package messagelisteners

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/streadway/amqp"
	logger "github.com/wso2/micro-gw/loggers"
	resourcetypes "github.com/wso2/micro-gw/pkg/resource_types"
)

// constant variables
const (
	apiEventType                = "API"
	applicationEventType        = "APPLICATION"
	subscriptionEventType       = "SUBSCRIPTIONS"
	scopeEvenType               = "SCOPE"
	removeAPIFromGateway        = "REMOVE_API_FROM_GATEWAY"
	deployAPIToGateway          = "DEPLOY_API_IN_GATEWAY"
	applicationRegistration     = "APPLICATION_REGISTRATION_CREATE"
	removeApplicationKeyMapping = "REMOVE_APPLICATION_KEYMAPPING"
)

// var variables
var (
	SubList                   = make([]resourcetypes.Subscription, 0)
	AppKeyMappingList         = make([]resourcetypes.ApplicationKeyMapping, 0)
	APIList                   = make([]resourcetypes.API, 0)
	ScopeList                 = make([]resourcetypes.Scope, 0)
	AppPolicyList             = make([]resourcetypes.ApplicationPolicy, 0)
	SubPolicyList             = make([]resourcetypes.SubscriptionPolicy, 0)
	ApplicationKeyMappingList = make([]resourcetypes.SubscriptionPolicy, 0)
	AppList                   = make([]resourcetypes.Application, 0)
	APIListTimeStamp          = make(map[string]int64, 0)
	ApplicationListTimeStamp  = make(map[string]int64, 0)
)

// handleNotification to process
func handleNotification(deliveries <-chan amqp.Delivery, done chan error) {
	for d := range deliveries {
		var notification EventNotification
		var eventType string
		json.Unmarshal([]byte(string(d.Body)), &notification)
		var decodedByte, err = base64.StdEncoding.DecodeString(notification.Event.PayloadData.Event)
		if err != nil {
			if _, ok := err.(base64.CorruptInputError); ok {
				panic("\nbase64 input is corrupt, check the provided key")
			}
			panic(err)
		}
		logger.LoggerJMS.Infof("\n\n[%s]", decodedByte)
		eventType = notification.Event.PayloadData.EventType

		if strings.Contains(eventType, apiEventType) {
			handleAPIEvents(decodedByte, eventType)
		} else if strings.Contains(eventType, applicationEventType) {
			handleApplicationEvents(decodedByte, eventType)
		} else if strings.Contains(eventType, subscriptionEventType) {
			handleSubscriptionEvents(decodedByte, eventType)
		} else if strings.Contains(eventType, scopeEvenType) {
			handleScopeEvents(decodedByte, eventType)
		} else {
			handlePolicyEvents(decodedByte, eventType)
		}
		d.Ack(false)
	}
	logger.LoggerJMS.Infof("handle: deliveries channel closed")
	done <- nil
}

// handleAPIEvents to process api related data
func handleAPIEvents(data []byte, eventType string) {
	var apiEvent APIEvent
	json.Unmarshal([]byte(string(data)), &apiEvent)
	timeStampList := APIListTimeStamp
	var oldTimeStamp int64 = 0
	var newTimeStamp int64 = apiEvent.Event.TimeStamp
	for apiID, timeStamp := range timeStampList {
		if strings.EqualFold(apiEvent.APIID, apiID) {
			oldTimeStamp = timeStamp
		} else {
			APIListTimeStamp[apiEvent.APIID] = newTimeStamp
		}
	}

	if strings.EqualFold(removeAPIFromGateway, apiEvent.Event.Type) && oldTimeStamp < newTimeStamp {
		for i := range APIList {
			if strings.EqualFold(apiEvent.APIID, APIList[i].APIID) {
				copy(APIList[i:], APIList[i+1:])
				APIList[len(APIList)-1] = resourcetypes.API{}
				APIList = APIList[:len(APIList)-1]
				break
			}
		}
	} else if strings.EqualFold(deployAPIToGateway, apiEvent.Event.Type) {
		// pull API details
		api := resourcetypes.API{APIID: apiEvent.APIID, Provider: apiEvent.APIProvider, Name: apiEvent.APIName,
			Version: apiEvent.APIVersion, Context: apiEvent.APIContext, APIType: apiEvent.APIType,
			IsDefaultVersion: true, TenantID: -1, TenantDomain: apiEvent.Event.TenantDomain,
			TimeStamp: apiEvent.Event.TimeStamp}
		APIList = append(APIList, api)
	}
}

// handleApplicationEvents to process application related events
func handleApplicationEvents(data []byte, eventType string) {
	if strings.EqualFold(applicationRegistration, eventType) ||
		strings.EqualFold(removeApplicationKeyMapping, eventType) {
		var applicationRegistrationEvent ApplicationRegistrationEvent
		json.Unmarshal([]byte(string(data)), &applicationRegistrationEvent)

		applicationKeyMapping := resourcetypes.ApplicationKeyMapping{ApplicationID: applicationRegistrationEvent.ApplicationID,
			ConsumerKey: applicationRegistrationEvent.ConsumerKey, KeyType: applicationRegistrationEvent.KeyType,
			KeyManager: applicationRegistrationEvent.KeyManager, TenantID: -1, TenantDomain: applicationRegistrationEvent.TenantDomain,
			TimeStamp: applicationRegistrationEvent.TimeStamp}

		AppKeyMappingList = append(AppKeyMappingList, applicationKeyMapping)
	} else {
		var applicationEvent ApplicationEvent
		json.Unmarshal([]byte(string(data)), &applicationEvent)
		application := resourcetypes.Application{UUID: applicationEvent.UUID, ID: applicationEvent.ApplicationID,
			Name: applicationEvent.ApplicationName, SubName: applicationEvent.Subscriber, Policy: applicationEvent.ApplicationPolicy, TokenType: applicationEvent.TokenType, GroupIds: applicationEvent.GroupID, Attributes: nil,
			TenantID: -1, TenantDomain: applicationEvent.TenantDomain, TimeStamp: applicationEvent.TimeStamp}

		AppList = append(AppList, application)
		// EventTypes: APPLICATION_CREATE, APPLICATION_UPDATE, APPLICATION_DELETE
	}
}

// handleSubscriptionRelatedEvents to process subscription related events
func handleSubscriptionEvents(data []byte, eventType string) {
	var subscriptionEvent SubscriptionEvent
	json.Unmarshal([]byte(string(data)), &subscriptionEvent)
	subscription := resourcetypes.Subscription{SubscriptionID: subscriptionEvent.SubscriptionID, PolicyID: subscriptionEvent.PolicyID,
		APIID: subscriptionEvent.APIID, AppID: subscriptionEvent.ApplicationID, SubscriptionState: subscriptionEvent.SubscriptionState,
		TenantID: subscriptionEvent.TenantID, TenantDomain: subscriptionEvent.TenantDomain, TimeStamp: subscriptionEvent.TimeStamp}

	SubList = append(SubList, subscription)
	// EventTypes: SUBSCRIPTIONS_CREATE, SUBSCRIPTIONS_UPDATE, SUBSCRIPTIONS_DELETE
}

// handleScopeRelatedEvents to process scope related events
func handleScopeEvents(data []byte, eventType string) {
	var scopeEvent ScopeEvent
	json.Unmarshal([]byte(string(data)), &scopeEvent)
	scope := resourcetypes.Scope{Name: scopeEvent.Name, DisplayName: scopeEvent.DisplayName, ApplicationName: scopeEvent.ApplicationName}
	ScopeList = append(ScopeList, scope)
	// EventTypes: SCOPE_CREATE, SCOPE_UPDATE,SCOPE_DELETE
}

// handlePolicyRelatedEvents to process policy related events
func handlePolicyEvents(data []byte, eventType string) {
	var policyEvent PolicyInfo
	json.Unmarshal([]byte(string(data)), &policyEvent)

	// TODO: Handle policy events
	if strings.EqualFold(eventType, "POLICY_CREATE") {
		logger.LoggerJMS.Infof("Policy: %s for policy type: %s", policyEvent.PolicyName, policyEvent.PolicyType)
	} else if strings.EqualFold(eventType, "POLICY_UPDATE") {
		logger.LoggerJMS.Infof("Policy: %s for policy type: %s", policyEvent.PolicyName, policyEvent.PolicyType)
	} else if strings.EqualFold(eventType, "POLICY_DELETE") {
		logger.LoggerJMS.Infof("Policy: %s for policy type: %s", policyEvent.PolicyName, policyEvent.PolicyType)
	}

	if strings.EqualFold(apiEventType, policyEvent.PolicyType) {
		var apiPolicyEvent APIPolicyEvent
		json.Unmarshal([]byte(string(data)), &apiPolicyEvent)
	} else if strings.EqualFold(applicationEventType, policyEvent.PolicyType) {
		applicationPolicy := resourcetypes.ApplicationPolicy{ID: policyEvent.PolicyID, TenantID: -1, Name: policyEvent.PolicyName,
			QuotaType: policyEvent.QuotaType}
		AppPolicyList = append(AppPolicyList, applicationPolicy)

	} else if strings.EqualFold(subscriptionEventType, policyEvent.PolicyType) {
		var subscriptionPolicyEvent SubscriptionPolicyEvent
		json.Unmarshal([]byte(string(data)), &subscriptionPolicyEvent)

		subscriptionPolicy := resourcetypes.SubscriptionPolicy{ID: subscriptionPolicyEvent.PolicyID, TenantID: -1,
			Name: subscriptionPolicyEvent.PolicyName, QuotaType: subscriptionPolicyEvent.QuotaType,
			GraphQLMaxComplexity: subscriptionPolicyEvent.GraphQLMaxComplexity,
			GraphQLMaxDepth:      subscriptionPolicyEvent.GraphQLMaxDepth, RateLimitCount: subscriptionPolicyEvent.RateLimitCount,
			RateLimitTimeUnit: subscriptionPolicyEvent.RateLimitTimeUnit, StopOnQuotaReach: subscriptionPolicyEvent.StopOnQuotaReach,
			TenantDomain: subscriptionPolicyEvent.TenantDomain, TimeStamp: subscriptionPolicyEvent.TimeStamp}

		SubPolicyList = append(SubPolicyList, subscriptionPolicy)
	}
}
