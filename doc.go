// Package httpsteps provides HTTP-related step definitions for github.com/cucumber/godog.
//
//		Feature: Example
//
//		 Scenario: Successful GET Request
//		   Given "template-service" receives "GET" request "/template/hello"
//
//		   And "template-service" responds with status "OK" and body
//		   """
//		   Hello, %s!
//		   """
//
//		   When I request HTTP endpoint with method "GET" and URI "/?name=Jane"
//
//		   Then I should have response with status "OK"
//
//		   And I should have response with body
//		   """
//		   Hello, Jane!
//		   """
package httpsteps
