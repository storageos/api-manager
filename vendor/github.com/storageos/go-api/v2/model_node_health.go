/*
 * StorageOS API
 *
 * No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)
 *
 * API version: 2.4.0-alpha
 * Contact: info@storageos.com
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package api
// NodeHealth The operational health of a node entity 
type NodeHealth string

// List of NodeHealth
const (
	NODEHEALTH_ONLINE NodeHealth = "online"
	NODEHEALTH_OFFLINE NodeHealth = "offline"
	NODEHEALTH_UNKNOWN NodeHealth = "unknown"
)
