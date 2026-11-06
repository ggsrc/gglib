// Package snowflake generates unique, roughly time-ordered 64-bit IDs with
// github.com/bwmarrin/snowflake, and gives each process its own node number
// from a Redis counter.
//
// A snowflake ID packs a millisecond timestamp, a 10-bit node number and a
// 12-bit sequence, so IDs are unique as long as no two running processes share
// a node number. NewNode takes the node number from INCR on
// "<appName>:snowflake:nodeID", modulo 1024: every process start takes the
// next number, and up to 1024 processes of one app can run at once.
//
// Init sets a default node for Generate, in the same way the zerolog/log
// package has a default logger.
package snowflake
