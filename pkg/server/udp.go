/*
 * Copyright (C) 2020-2022, IrineSistiana
 *
 * This file is part of mosdns.
 *
 * mosdns is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mosdns is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"

	"github.com/IrineSistiana/mosdns/v5/pkg/pool"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

const (
	FastActionContinue = iota
	FastActionReply
)

type UDPServerOpts struct {
	Logger     *zap.Logger
	FastBypass func(reqLen int, buf []byte, clientAddr netip.AddrPort) (action int, respLen int, preMarks uint64, preDset string)
	// FastRaw can return a DNS rcode for a complete, stateless raw-query plan.
	// It is called only for a bare single-question DNS query without sections or
	// EDNS records, allowing the response to be written in the receive buffer.
	FastRaw func(qname string) (rcode int, ok bool)
	// WorkerCount enables a bounded worker queue for packet handling. A zero
	// value preserves the historical one-goroutine-per-query behavior.
	WorkerCount int
	// Inline handles packets in the read loop. It is intended for handlers that
	// are known to be CPU-only and never wait for upstream I/O.
	Inline bool
}

type udpJob struct {
	q           *dns.Msg
	meta        QueryMeta
	remoteAddr  netip.AddrPort
	dstIPFromCM net.IP
}

func ServeUDP(c *net.UDPConn, h Handler, opts UDPServerOpts) error {
	logger := opts.Logger
	if logger == nil {
		logger = nopLogger
	}

	listenerCtx, cancel := context.WithCancelCause(context.Background())
	defer cancel(errListenerCtxCanceled)

	rb := pool.GetBuf(dns.MaxMsgSize)
	defer pool.ReleaseBuf(rb)

	oobReader, oobWriter, err := initOobHandler(c)
	if err != nil {
		return fmt.Errorf("failed to init oob handler, %w", err)
	}
	var ob []byte
	if oobReader != nil {
		obp := pool.GetBuf(1024)
		defer pool.ReleaseBuf(obp)
		ob = *obp
	}
	// When the socket is bound to a specific address, no destination-address
	// control message is needed. Use the lighter recvfrom/sendto path instead
	// of recvmsg/sendmsg with an empty ancillary-data buffer.
	simpleUDP := oobReader == nil && oobWriter == nil

	var jobs chan udpJob
	if opts.WorkerCount > 0 {
		workerCount := opts.WorkerCount
		if workerCount > 1024 {
			workerCount = 1024
		}
		jobs = make(chan udpJob, workerCount*4)
		for i := 0; i < workerCount; i++ {
			go func() {
				for job := range jobs {
					handleUDPJob(c, h, listenerCtx, logger, simpleUDP, oobWriter, job)
				}
			}()
		}
		defer close(jobs)
	}

	for {
		var n, oobn int
		var remoteAddr netip.AddrPort
		var err error
		if simpleUDP {
			n, remoteAddr, err = c.ReadFromUDPAddrPort(*rb)
		} else {
			n, oobn, _, remoteAddr, err = c.ReadMsgUDPAddrPort(*rb, ob)
		}
		if err != nil {
			if n == 0 {
				return fmt.Errorf("unexpected read err: %w", err)
			}
			logger.Warn("read err", zap.Error(err))
			continue
		}

		var dstIpFromCm net.IP
		if oobReader != nil {
			dstIpFromCm, _ = oobReader(ob[:oobn])
		}

		var preMarks uint64
		var preDset string
		if opts.FastBypass != nil {
			action, respLen, marks, dset := opts.FastBypass(n, *rb, remoteAddr)
			if action == FastActionReply {
				if respLen > 0 {
					if simpleUDP {
						_, _ = c.WriteToUDPAddrPort((*rb)[:respLen], remoteAddr)
					} else {
						var oob []byte
						if oobWriter != nil && dstIpFromCm != nil {
							oob = oobWriter(dstIpFromCm)
						}
						_, _, _ = c.WriteMsgUDPAddrPort((*rb)[:respLen], oob, remoteAddr)
					}
				}
				continue
			}
			preMarks = marks
			preDset = dset
		}

		// FastRaw is deliberately evaluated after FastBypass. Any precomputed
		// switch/domain state takes precedence and falls back to the full path.
		if opts.FastRaw != nil && preMarks == 0 && preDset == "" {
			if qname, questionEnd, ok := parseBareDNSQuestion((*rb)[:n]); ok {
				if rcode, matched := opts.FastRaw(qname); matched {
					respLen := makeRawReject((*rb)[:n], questionEnd, rcode)
					if simpleUDP {
						_, _ = c.WriteToUDPAddrPort((*rb)[:respLen], remoteAddr)
					} else {
						var oob []byte
						if oobWriter != nil && dstIpFromCm != nil {
							oob = oobWriter(dstIpFromCm)
						}
						_, _, _ = c.WriteMsgUDPAddrPort((*rb)[:respLen], oob, remoteAddr)
					}
					continue
				}
			}
		}

		q := new(dns.Msg)
		if err := q.Unpack((*rb)[:n]); err != nil {
			logger.Warn("invalid msg", zap.Error(err), zap.Binary("msg", (*rb)[:n]), zap.Stringer("from", remoteAddr))
			continue
		}

		meta := QueryMeta{
			ClientAddr:       remoteAddr.Addr(),
			FromUDP:          true,
			PreFastFlags:     preMarks,
			PreFastDomainSet: preDset,
		}
		// Consume+strip the private client-IP option (audit only; leaves the
		// transport ClientAddr and everything downstream unchanged).
		if real, ok := ExtractClientIP(q, remoteAddr.Addr()); ok {
			meta.RealClientAddr = real
		}

		job := udpJob{q: q, meta: meta, remoteAddr: remoteAddr, dstIPFromCM: dstIpFromCm}
		if jobs != nil {
			jobs <- job
		} else if opts.Inline {
			handleUDPJob(c, h, listenerCtx, logger, simpleUDP, oobWriter, job)
		} else {
			go handleUDPJob(c, h, listenerCtx, logger, simpleUDP, oobWriter, job)
		}
	}
}

func handleUDPJob(c *net.UDPConn, h Handler, listenerCtx context.Context, logger *zap.Logger, simpleUDP bool, oobWriter writeSrcAddrToOOB, job udpJob) {
	payload := h.Handle(listenerCtx, job.q, job.meta, pool.PackBuffer)
	if payload == nil {
		return
	}
	defer pool.ReleaseBuf(payload)

	if simpleUDP {
		if _, err := c.WriteToUDPAddrPort(*payload, job.remoteAddr); err != nil {
			logger.Warn("failed to write response", zap.Stringer("client", job.remoteAddr), zap.Error(err))
		}
		return
	}

	var oob []byte
	if oobWriter != nil && job.dstIPFromCM != nil {
		oob = oobWriter(job.dstIPFromCM)
	}
	if _, _, err := c.WriteMsgUDPAddrPort(*payload, oob, job.remoteAddr); err != nil {
		logger.Warn("failed to write response", zap.Stringer("client", job.remoteAddr), zap.Error(err))
	}
}

type getSrcAddrFromOOB func(oob []byte) (net.IP, error)
type writeSrcAddrToOOB func(a net.IP) []byte

// parseBareDNSQuestion parses the subset of DNS packets that can be answered
// without allocating a dns.Msg. The conservative section-count checks keep
// EDNS, TSIG and malformed packets on the normal parser path.
func parseBareDNSQuestion(buf []byte) (qname string, questionEnd int, ok bool) {
	if len(buf) < 17 {
		return "", 0, false
	}
	if binary.BigEndian.Uint16(buf[4:6]) != 1 ||
		binary.BigEndian.Uint16(buf[6:8]) != 0 ||
		binary.BigEndian.Uint16(buf[8:10]) != 0 ||
		binary.BigEndian.Uint16(buf[10:12]) != 0 {
		return "", 0, false
	}

	var name [256]byte
	nameLen := 0
	offset := 12
	for {
		if offset >= len(buf) {
			return "", 0, false
		}
		labelLen := int(buf[offset])
		offset++
		if labelLen == 0 {
			if nameLen == 0 {
				name[0] = '.'
				nameLen = 1
			}
			break
		}
		if labelLen > 63 || offset+labelLen > len(buf) || nameLen+labelLen+1 >= len(name) {
			return "", 0, false
		}
		copy(name[nameLen:], buf[offset:offset+labelLen])
		nameLen += labelLen
		name[nameLen] = '.'
		nameLen++
		offset += labelLen
	}

	if offset+4 != len(buf) {
		return "", 0, false
	}
	return string(name[:nameLen]), offset + 4, true
}

func makeRawReject(buf []byte, questionEnd int, rcode int) int {
	flags := binary.BigEndian.Uint16(buf[2:4])
	// SetReply copies only opcode/RD/CD; EntryHandler also enables RA.
	flags &= 0x7910 // opcode, RD and CD
	flags |= 0x8080 // QR and RA
	flags = (flags & 0xfff0) | uint16(rcode&0x0f)
	binary.BigEndian.PutUint16(buf[2:4], flags)
	for i := 6; i < 12; i++ {
		buf[i] = 0
	}
	return questionEnd
}
