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

package sequence

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"go.uber.org/zap"
)

const PluginType = "sequence"

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })

	MustRegExecQuickSetup("accept", setupAccept)
	MustRegExecQuickSetup("reject", setupReject)
	MustRegExecQuickSetup("return", setupReturn)
	MustRegExecQuickSetup("goto", setupGoto)
	MustRegExecQuickSetup("jump", setupJump)
	MustRegExecQuickSetup("exit", setupExit)
	MustRegExecQuickSetup("try", setupTry)
	MustRegMatchQuickSetup("_true", setupTrue)
	MustRegMatchQuickSetup("_false", setupFalse)
}

type fastMatcher interface {
	GetFastCheck() func(qCtx *query_context.Context) bool
}
type fastExecutor interface {
	GetFastExec() func(ctx context.Context, qCtx *query_context.Context) error
}

type rawQNameMatcher interface {
	MatchQName(qname string) bool
}

type rawRejectRule struct {
	match  func(string) bool
	rcode  int
	always bool
}

type Sequence struct {
	chain            []*ChainNode
	anonymousPlugins[]any
	logger           *zap.Logger
	isInline         bool
	instructions[]instruction
	rawRejectRules  []rawRejectRule
	rawRejectReady  bool
}

func (s *Sequence) Close() error {
	for _, plugin := range s.anonymousPlugins {
		closePlugin(plugin)
	}
	return nil
}

type Args =[]RuleArgs

func Init(bp *coremain.BP, args any) (any, error) {
	return NewSequence(NewBQ(bp.M(), bp.L()), *args.(*Args))
}

func NewSequence(bq BQ, ra[]RuleArgs) (*Sequence, error) {
	s := &Sequence{
		logger: bq.L(),
	}

	var rc[]RuleConfig
	for _, ra := range ra {
		rc = append(rc, parseArgs(ra))
	}
	if err := s.buildChain(bq, rc); err != nil {
		_ = s.Close()
		return nil, err
	}
	s.compile()
	return s, nil
}

func (s *Sequence) compile() {
	var ins[]instruction
	for _, node := range s.chain {
		instr := instruction{node: node}

		// 1. 检查 Matchers 是否全都可以优化
		allMatchersOptimizable := true
		var checks[]func(qCtx *query_context.Context) bool

		for _, m := range node.Matches {
			if fm, ok := m.Matcher.(fastMatcher); ok {
				if check := fm.GetFastCheck(); check != nil {
					name := m.Name

					// [终极优化] 预先识别哪些 Matcher 需要附加副作用 (KeyDomainSet)
					isSwitch6 := strings.HasPrefix(name, "anonymous_match(switch6:")
					isSwitch5 := strings.HasPrefix(name, "anonymous_match(switch5:")
					isQnameDollar := strings.HasPrefix(name, "anonymous_match(qname: $")
					isQnameNormal := strings.HasPrefix(name, "anonymous_match(qname: ")

					if isSwitch6 || isSwitch5 || isQnameDollar || isQnameNormal {
						// 预处理 qname 规则名，省去执行时的字符串截取开销
						var ruleName string
						if isQnameDollar {
							ruleName = strings.TrimSuffix(strings.TrimPrefix(name, "anonymous_match(qname: $"), ")")
						} else if isQnameNormal {
							ruleName = strings.TrimSuffix(strings.TrimPrefix(name, "anonymous_match(qname: "), ")")
						}

						// 构造一个包装了打标签逻辑的“超级闭包”，完美留在快路径中！
						wrappedCheck := func(qCtx *query_context.Context) bool {
							matched := check(qCtx)
							if matched {
								if _, exists := qCtx.GetValue(query_context.KeyDomainSet); !exists {
									if isSwitch6 {
										if len(qCtx.Q().Question) > 0 && qCtx.Q().Question[0].Qtype == 28 {
											qCtx.StoreValue(query_context.KeyDomainSet, "BANAAAA")
										}
									} else if isSwitch5 {
										if len(qCtx.Q().Question) > 0 {
											qtype := qCtx.Q().Question[0].Qtype
											var domainSetName string
											switch qtype {
											case 6: // dns.TypeSOA
												domainSetName = "BANSOA"
											case 12: // dns.TypePTR
												domainSetName = "BANPTR"
											case 65: // dns.TypeHTTPS
												domainSetName = "BANHTTPS"
											}
											if domainSetName != "" {
												qCtx.StoreValue(query_context.KeyDomainSet, domainSetName)
											}
										}
									} else if ruleName != "" {
										qCtx.StoreValue(query_context.KeyDomainSet, ruleName)
									}
								}
							}
							return matched
						}
						checks = append(checks, wrappedCheck)
					} else {
						// 没有副作用的纯净检查器，直接加入
						checks = append(checks, check)
					}
					continue
				}
			}
			// 如果走到这里，说明该插件本身不支持快路径（没有实现 GetFastCheck）
			allMatchersOptimizable = false
			break
		}

		// 2. 检查 Executor 是否可以优化
		var fExec func(context.Context, *query_context.Context) error
		if allMatchersOptimizable {
			if node.E != nil {
				if fe, ok := node.E.(fastExecutor); ok {
					fExec = fe.GetFastExec()
				}
			} else if node.RE != nil {
				if fe, ok := node.RE.(fastExecutor); ok {
					fExec = fe.GetFastExec()
					instr.isTerminal = true
				}
			}
		}

		// 3. 标记 Simple 路径
		if allMatchersOptimizable && fExec != nil {
			instr.isSimple = true
			instr.fastChecks = checks
			instr.fastExec = fExec
		}
		ins = append(ins, instr)
	}
	s.instructions = ins
	s.compileRawReject()
}

// compileRawReject recognizes a complete, stateless reject sequence. Such a
// sequence can answer a bare DNS question directly from the UDP receive buffer
// without constructing dns.Msg, query_context, or a packed response.
func (s *Sequence) compileRawReject() {
	s.rawRejectRules = nil
	s.rawRejectReady = false
	if len(s.instructions) == 0 {
		return
	}

	rules := make([]rawRejectRule, 0, len(s.instructions))
	for _, instr := range s.instructions {
		if !instr.isSimple || instr.node.RE == nil || len(instr.node.Matches) != 1 {
			return
		}
		action, ok := instr.node.RE.(ActionReject)
		if !ok {
			if actionPtr, ptrOK := instr.node.RE.(*ActionReject); ptrOK && actionPtr != nil {
				action = *actionPtr
				ok = true
			}
		}
		if !ok || action.Rcode < 0 || action.Rcode > 15 {
			return
		}

		matcher := instr.node.Matches[0].Matcher
		rule := rawRejectRule{rcode: action.Rcode}
		switch matcher.(type) {
		case MatchAlwaysTrue, *MatchAlwaysTrue:
			rule.always = true
			rule.match = func(string) bool { return true }
		default:
			rawMatcher, rawOK := matcher.(rawQNameMatcher)
			if !rawOK {
				return
			}
			rule.match = rawMatcher.MatchQName
		}
		rules = append(rules, rule)
	}

	if !rules[len(rules)-1].always {
		return
	}
	s.rawRejectRules = rules
	s.rawRejectReady = true
}

// RawReject evaluates the compiled stateless reject plan for a normalized or
// fully-qualified qname. The final _true rule makes a ready plan total.
func (s *Sequence) RawReject(qname string) (rcode int, ok bool) {
	if !s.rawRejectReady {
		return 0, false
	}
	for _, rule := range s.rawRejectRules {
		if rule.match(qname) {
			return rule.rcode, true
		}
	}
	return 0, false
}

func (s *Sequence) RawRejectComplete() bool {
	return s.rawRejectReady
}

// Exec 执行序列逻辑
func (s *Sequence) Exec(ctx context.Context, qCtx *query_context.Context) error {
	// 组装并启动已经进化完毕的高性能 Walker
	walker := NewChainWalker(s.instructions, s.chain, nil, s.logger)
	return walker.ExecNext(ctx, qCtx)
}

// IsInlineSafe reports whether this sequence contains only the small built-in
// terminal actions that never wait for network or other external I/O. UDP
// servers can safely run such a sequence in their read loop and avoid a
// goroutine per packet.
func (s *Sequence) IsInlineSafe() bool {
	if len(s.instructions) == 0 {
		return false
	}
	for _, instr := range s.instructions {
		if !instr.isSimple || instr.node.RE == nil {
			return false
		}
		switch instr.node.RE.(type) {
		case ActionAccept, ActionReject, ActionExit:
		default:
			return false
		}
	}
	return true
}

func (s *Sequence) buildChain(bq BQ, rs []RuleConfig) error {
	c := make([]*ChainNode, 0, len(rs))
	for ri, r := range rs {
		n, err := s.newNode(bq, r, ri)
		if err != nil {
			return fmt.Errorf("failed to init rule #%d, %w", ri, err)
		}
		c = append(c, n)
	}
	s.chain = c
	return nil
}

func (s *Sequence) newNode(bq BQ, r RuleConfig, ri int) (*ChainNode, error) {
	n := new(ChainNode)

	// PluginName generation
	if len(r.Execs) == 0 {
		n.PluginName = "no_exec"
	} else if len(r.Execs) == 1 {
		ec := r.Execs[0]
		if len(ec.Tag) > 0 {
			n.PluginName = ec.Tag
		} else {
			n.PluginName = fmt.Sprintf("anonymous_exec(%s: %v)", ec.Type, ec.Args)
		}
	} else {
		var names[]string
		for _, ec := range r.Execs {
			if len(ec.Tag) > 0 {
				names = append(names, ec.Tag)
			} else {
				names = append(names, ec.Type)
			}
		}
		n.PluginName = fmt.Sprintf("multi_exec[%s]", strings.Join(names, ","))
	}

	// init matches
	for mi, mc := range r.Matches {
		namedM, err := s.newMatcher(bq, mc, ri, mi)
		if err != nil {
			return nil, fmt.Errorf("failed to init matcher #%d, %w", mi, err)
		}
		n.Matches = append(n.Matches, namedM)
	}

	// init exec
	if len(r.Execs) == 1 {
		e, re, err := s.newExec(bq, r.Execs[0], ri)
		if err != nil {
			return nil, fmt.Errorf("failed to init exec, %w", err)
		}
		n.E = e
		n.RE = re
	} else if len(r.Execs) > 1 {
		var subRules[]RuleConfig
		for _, ec := range r.Execs {
			subRules = append(subRules, RuleConfig{
				Execs:[]ExecConfig{ec},
			})
		}

		// [修改] 创建子序列时，标记 isInline = true
		subSeq := &Sequence{
			logger:   s.logger,
			isInline: true,
		}

		if err := subSeq.buildChain(bq, subRules); err != nil {
			return nil, fmt.Errorf("failed to build multi-exec sequence: %w", err)
		}

		subSeq.compile()
		s.anonymousPlugins = append(s.anonymousPlugins, subSeq)
		n.E = subSeq
	}

	return n, nil
}

func (s *Sequence) newMatcher(bq BQ, mc MatchConfig, ri, mi int) (NamedMatcher, error) {
	var m Matcher
	var name string

	switch {
	case len(mc.Tag) > 0:
		name = mc.Tag
		p, _ := bq.M().GetPlugin(name).(Matcher)
		if p == nil {
			return NamedMatcher{}, fmt.Errorf("can not find matcher %s", name)
		}
		if qc, ok := p.(QuickConfigurableMatch); ok {
			v, err := qc.QuickConfigureMatch(mc.Args)
			if err != nil {
				return NamedMatcher{}, fmt.Errorf("fail to configure plugin %s, %w", name, err)
			}
			m = v
		} else {
			m = p
		}

	case len(mc.Type) > 0:
		name = fmt.Sprintf("anonymous_match(%s: %v)", mc.Type, mc.Args)
		f := GetMatchQuickSetup(mc.Type)
		if f == nil {
			return NamedMatcher{}, fmt.Errorf("invalid matcher type %s", mc.Type)
		}
		p, err := f(NewBQ(bq.M(), bq.L().Named(fmt.Sprintf("r%d.m%d", ri, mi))), mc.Args)
		if err != nil {
			return NamedMatcher{}, fmt.Errorf("failed to init matcher, %w", err)
		}
		s.anonymousPlugins = append(s.anonymousPlugins, p)
		m = p
	default:
		return NamedMatcher{}, fmt.Errorf("missing args")
	}

	if mc.Reverse {
		m = reverseMatcher(m)
		name = fmt.Sprintf("not(%s)", name)
	}
	return NamedMatcher{Name: name, Matcher: m}, nil
}

func (s *Sequence) newExec(bq BQ, ec ExecConfig, ri int) (Executable, RecursiveExecutable, error) {
	var exec any
	switch {
	case len(ec.Tag) > 0:
		p := bq.M().GetPlugin(ec.Tag)
		if p == nil {
			return nil, nil, fmt.Errorf("can not find executable %s", ec.Tag)
		}
		if qc, ok := p.(QuickConfigurableExec); ok {
			v, err := qc.QuickConfigureExec(ec.Args)
			if err != nil {
				return nil, nil, fmt.Errorf("fail to configure plugin %s, %w", ec.Tag, err)
			}
			exec = v
		} else {
			exec = p
		}
	case len(ec.Type) > 0:
		f := GetExecQuickSetup(ec.Type)
		if f == nil {
			return nil, nil, fmt.Errorf("invalid executable type %s", ec.Type)
		}
		v, err := f(NewBQ(bq.M(), bq.L().Named(fmt.Sprintf("r%d", ri))), ec.Args)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init executable, %w", err)
		}
		s.anonymousPlugins = append(s.anonymousPlugins, v)
		exec = v
	default:
		return nil, nil, fmt.Errorf("missing args")
	}

	e, _ := exec.(Executable)
	re, _ := exec.(RecursiveExecutable)

	if re == nil && e == nil {
		return nil, nil, fmt.Errorf("invalid args, initialized object is not executable")
	}
	return e, re, nil
}

func closePlugin(p any) {
	if c, ok := p.(io.Closer); ok {
		_ = c.Close()
	}
}

func reverseMatcher(m Matcher) Matcher {
	return reverseMatch{m: m}
}

type reverseMatch struct {
	m Matcher
}

func (r reverseMatch) Match(ctx context.Context, qCtx *query_context.Context) (bool, error) {
	ok, err := r.m.Match(ctx, qCtx)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func (r reverseMatch) GetFastCheck() func(qCtx *query_context.Context) bool {
	if fm, ok := r.m.(fastMatcher); ok {
		if innerCheck := fm.GetFastCheck(); innerCheck != nil {
			return func(qCtx *query_context.Context) bool {
				return !innerCheck(qCtx) 
			}
		}
	}
	return nil
}

func (s *Sequence) GetFastExec() func(context.Context, *query_context.Context) error {
	return s.Exec
}
