//go:build ebpf
// +build ebpf

// SPDX-License-Identifier: GPL-2.0
// O3K eBPF Security Groups - XDP Packet Filter
// Copyright 2026 O3K Project

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/icmp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Security group rule structure (must match Go struct)
struct sg_rule {
	__u8 protocol;           // IPPROTO_TCP, IPPROTO_UDP, IPPROTO_ICMP, 0=any
	__u8 direction;          // 0=ingress, 1=egress
	__u16 port_min;          // Minimum port (host byte order)
	__u16 port_max;          // Maximum port (host byte order)
	__u32 remote_ip_prefix;  // CIDR prefix (network byte order)
	__u32 remote_ip_mask;    // CIDR mask (network byte order)
} __attribute__((packed));

// Security group rule set (max 100 rules per port)
struct sg_rule_set {
	__u32 rule_count;
	struct sg_rule rules[100];
} __attribute__((packed));

// BPF map: port_id (hash of MAC address) -> security group rules
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 10000);
	__type(key, __u32);
	__type(value, struct sg_rule_set);
} sg_rules SEC(".maps");

// BPF map: statistics (packet counters)
struct sg_stats {
	__u64 packets_allowed;
	__u64 packets_denied;
	__u64 packets_processed;
};

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 1);
	__type(key, __u32);
	__type(value, struct sg_stats);
} sg_statistics SEC(".maps");

// Simple hash function for MAC address -> port_id
static __always_inline __u32 mac_to_port_id(unsigned char *mac) {
	__u32 hash = 0;
	for (int i = 0; i < 6; i++) {
		hash = (hash * 31) + mac[i];
	}
	return hash;
}

// Check if IP matches CIDR
static __always_inline int ip_matches_cidr(__u32 ip, __u32 prefix, __u32 mask) {
	return (ip & mask) == (prefix & mask);
}

// Parse and check TCP/UDP port
static __always_inline int check_port(void *data, void *data_end, __u8 protocol,
                                      __u16 port_min, __u16 port_max) {
	if (protocol == IPPROTO_TCP) {
		struct tcphdr *tcp = data;
		if ((void *)(tcp + 1) > data_end)
			return 0;

		__u16 dport = bpf_ntohs(tcp->dest);
		return (dport >= port_min && dport <= port_max);
	} else if (protocol == IPPROTO_UDP) {
		struct udphdr *udp = data;
		if ((void *)(udp + 1) > data_end)
			return 0;

		__u16 dport = bpf_ntohs(udp->dest);
		return (dport >= port_min && dport <= port_max);
	}

	return 1; // No port check for other protocols
}

SEC("xdp")
int xdp_security_group_filter(struct xdp_md *ctx) {
	void *data_end = (void *)(long)ctx->data_end;
	void *data = (void *)(long)ctx->data;

	// Parse Ethernet header
	struct ethhdr *eth = data;
	if ((void *)(eth + 1) > data_end)
		return XDP_DROP;

	// Only process IPv4 packets
	if (eth->h_proto != bpf_htons(ETH_P_IP))
		return XDP_PASS;

	// Parse IP header
	struct iphdr *ip = (void *)(eth + 1);
	if ((void *)(ip + 1) > data_end)
		return XDP_DROP;

	// Get port ID from destination MAC address
	__u32 port_id = mac_to_port_id(eth->h_dest);

	// Lookup security group rules for this port
	struct sg_rule_set *rules = bpf_map_lookup_elem(&sg_rules, &port_id);
	if (!rules) {
		// No rules configured - default deny
		__u32 zero = 0;
		struct sg_stats *stats = bpf_map_lookup_elem(&sg_statistics, &zero);
		if (stats) {
			__sync_fetch_and_add(&stats->packets_denied, 1);
			__sync_fetch_and_add(&stats->packets_processed, 1);
		}
		return XDP_DROP;
	}

	// Get transport layer header pointer
	void *l4_hdr = (void *)ip + sizeof(*ip);

	// Check each security group rule
	#pragma unroll
	for (int i = 0; i < 100; i++) {
		if (i >= rules->rule_count)
			break;

		struct sg_rule *rule = &rules->rules[i];

		// Match protocol (0 = any protocol)
		if (rule->protocol != 0 && rule->protocol != ip->protocol)
			continue;

		// Match source IP against CIDR
		if (!ip_matches_cidr(ip->saddr, rule->remote_ip_prefix, rule->remote_ip_mask))
			continue;

		// Match port range (for TCP/UDP)
		if (rule->protocol == IPPROTO_TCP || rule->protocol == IPPROTO_UDP) {
			if (!check_port(l4_hdr, data_end, rule->protocol, rule->port_min, rule->port_max))
				continue;
		}

		// Rule matched - ACCEPT
		__u32 zero = 0;
		struct sg_stats *stats = bpf_map_lookup_elem(&sg_statistics, &zero);
		if (stats) {
			__sync_fetch_and_add(&stats->packets_allowed, 1);
			__sync_fetch_and_add(&stats->packets_processed, 1);
		}
		return XDP_PASS;
	}

	// No rules matched - DROP
	__u32 zero = 0;
	struct sg_stats *stats = bpf_map_lookup_elem(&sg_statistics, &zero);
	if (stats) {
		__sync_fetch_and_add(&stats->packets_denied, 1);
		__sync_fetch_and_add(&stats->packets_processed, 1);
	}
	return XDP_DROP;
}

char _license[] SEC("license") = "GPL";
