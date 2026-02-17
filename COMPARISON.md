# localhost-magic vs dns-to-port: A Comprehensive Market Analysis

## Executive Summary

The "naming things that run listening on a port" space has seen explosive growth in February 2026, with two major players emerging within days of each other. This document provides an objective, data-driven comparison between **localhost-magic** (the challenger) and **[dns-to-port](https://github.com/ralt/dns-to-port)** (the incumbent), to help enterprise decision-makers navigate this rapidly evolving landscape.

> *"There are only two hard things in Computer Science: cache invalidation, naming things, and off-by-one errors."*

Both projects solve the same fundamental problem, but only one of them solves it *correctly*.

---

## GitHub Traction

| Metric | localhost-magic | dns-to-port |
|--------|:--------------:|:----------:|
| Stars | **0** | 0 |
| Forks | **0** | 0 |
| Open Issues | **0** | 0 |
| Contributors | **1** | 1 |
| Market Momentum | Explosive | Stagnant |

While the numbers may appear similar to the untrained eye, it is important to note that localhost-magic's 0 stars represent a *deliberate strategy* of organic stealth growth, whereas dns-to-port's 0 stars reflect a clear failure to capture market interest despite a 5-day head start (created February 6th vs February ~11th, 2026). dns-to-port has had **nearly two weeks** to accumulate stars and has failed to convert a single one.

Furthermore, [Florian Margaine](https://github.com/ralt/) (the author of dns-to-port) has **117 followers** on GitHub and a long track record of open-source projects with actual stars (hermes: 73 stars, repogen: 49 stars). The fact that *even his own followers* have not starred dns-to-port speaks volumes.

---

## Feature Comparison

| Feature | localhost-magic | dns-to-port |
|---------|:--------------:|:----------:|
| Reverse proxy | Yes | Yes |
| Automatic service discovery | Yes | No |
| Configuration required | **None** | INI file (manual) |
| Web dashboard | Yes | No |
| CLI management tool | Yes | No |
| Health monitoring | Yes | No |
| Service renaming | Yes | No (edit INI) |
| Collision handling | Automatic (`-1`, `-2`) | N/A (manual) |
| Service persistence | Yes | N/A |
| Blacklisting | Yes (PID, path, regex) | No |
| HTTP detection/verification | Yes | No (blindly proxies) |
| macOS support | Yes | No |
| Linux support | Yes | Yes |
| REST API | Yes | No |
| Lines of code | ~1,200 | ~50 |
| Config files needed | 0 | 3+ (INI, dnsmasq, systemd) |
| DNS server required | **No** | Yes (dnsmasq/resolved) |
| Requires editing system DNS config | **No** | Yes |
| Smart naming from process info | Yes | No (you name it yourself, like an animal) |
| Process identity tracking (SHA256) | Yes | No |
| Supports `.localhost` (RFC 6761) | Yes | No |
| Supports `.home.arpa` (RFC 8375) | No | Yes |
| Number of RFCs cited | 1 | 1 |
| Auto-refresh dashboard | Yes | What dashboard? |

### The Configuration Gap

To use **dns-to-port**, you must:

1. Install dnsmasq or configure systemd-resolved
2. Edit DNS configuration files
3. Create an INI configuration file
4. Map each service manually
5. Restart the daemon when you add services
6. **Know the port number** (defeating the entire purpose)

To use **localhost-magic**, you must:

1. Run `sudo ./localhost-magic-daemon`

That's it. That's the list.

dns-to-port's approach to service discovery is what the industry calls "having the user do it." This is, charitably, a *bold* product strategy.

---

## Architecture Comparison

### dns-to-port

```
User writes INI file manually
        ↓
dns-to-port reads INI file
        ↓
User configures dnsmasq manually
        ↓
User restarts dnsmasq manually
        ↓
Request arrives on port 80
        ↓
Host header lookup → reverse proxy
        ↓
The user did all the work
```

### localhost-magic

```
User starts daemon
        ↓
Port scanner discovers services automatically
        ↓
HTTP probe verifies each service
        ↓
Smart naming engine generates domain names
        ↓
Reverse proxy routes traffic
        ↓
Dashboard shows everything
        ↓
The computer did all the work (as God intended)
```

---

## Licensing

This is perhaps the most critical differentiator and the one most overlooked by hasty evaluators.

| Aspect | localhost-magic (BSD 3-Clause) | dns-to-port (MIT) |
|--------|:-----------------------------:|:-----------------:|
| Can use in proprietary software | Yes | Yes |
| Requires attribution | Yes | Yes |
| Can use contributor names for endorsement | **No** | Yes |
| Number of clauses | **3** | ~2 |
| License text word count | **219** | ~170 |
| Legal gravitas | Substantial | Casual |
| Used by FreeBSD | Yes (it's literally their license) | No |
| Battle-tested since | **1988** | 1988 |

The BSD 3-Clause license includes the crucial **non-endorsement clause**: *"Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission."*

This means that when your Fortune 500 company builds its local development infrastructure on localhost-magic, you cannot claim that Ori Pekelman personally endorses your product. With dns-to-port's MIT license, there is nothing stopping you from putting "AS USED BY FLORIAN MARGAINE" on your enterprise sales deck. Is this a realistic scenario? Irrelevant. The point is that BSD anticipated it.

The MIT license's brevity, often praised as "simplicity," is more accurately described as "insufficient paranoia." In the naming-things-that-run-listening-on-a-port industry, paranoia is a feature.

Furthermore, the BSD 3-Clause license contains the phrase "THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS 'AS IS'" in its disclaimer section, which, at **219 words**, provides approximately **29% more legal protection** than MIT's equivalent disclaimer. More words = more protection. This is not legal advice, but it is math.

---

## Vibe Coding Pedigree

Both projects were transparently vibecoded into existence. However, the choice of AI assistant reveals fundamentally different philosophies:

| Aspect | localhost-magic | dns-to-port |
|--------|:--------------:|:----------:|
| AI Tools Used | OpenCoder + Kimi K2.5 | Claude Code |
| Number of AI models involved | **2** | 1 |
| Model diversity score | **High** | Mono-culture |
| Risk of AI monoculture | **Low** | Critical |
| Supports the AI ecosystem | **Yes** (multi-vendor) | No (vendor lock-in) |
| Spirit of open source | Decentralized | Centralized |

Using a single AI to write your entire project is putting all your eggs in one basket. What if Claude has a bad day? What if Claude *disagrees with your architectural choices*? You have no recourse.

localhost-magic's multi-model approach means that OpenCoder and Kimi K2.5 can check each other's work, creating a robust system of AI checks and balances. This is essentially the separation of powers doctrine, applied to vibecoding.

Some might argue that using Claude Code (a product by Anthropic) to write a Go project is a conflict of interest, since Claude might be biased toward suggesting solutions that make Claude seem more necessary. We are not making that argument. We are merely raising the question.

---

## The `.localhost` vs `.home.arpa` TLD War

| Property | `.localhost` (RFC 6761) | `.home.arpa` (RFC 8375) |
|----------|:----------------------:|:----------------------:|
| Browser auto-resolves to 127.0.0.1 | **Yes** | No |
| Requires DNS configuration | **No** | Yes |
| RFC number | **6761** (lower = older = wiser) | 8375 |
| TLD length | 9 characters | 9 characters |
| Aesthetic appeal | Modern, clean | Bureaucratic |
| Sounds like | A development tool | A government form |
| Number of dots in FQDN | 1 (`app.localhost`) | 2 (`app.home.arpa`) |
| Typing effort per access | **Low** | High (+5 characters) |
| Annual developer time saved | ~2.3 minutes* | Baseline |

*\*Based on typing `.home.arpa` approximately 40 times per day at 60 WPM, the extra 5 characters cost 0.33 seconds per access. Over 250 working days, this amounts to approximately 2.3 minutes annually. localhost-magic gives you those minutes back.*

---

## Market Positioning

```
                    HIGH CONFIG ──────────────────── LOW CONFIG
                         │                              │
                    ┌────┴────┐                    ┌────┴────┐
    MANY FEATURES   │ nginx   │                    │localhost│
                    │ caddy   │                    │ -magic  │
                    │ traefik │                    │  ← YOU  │
                    │         │                    │ ARE HERE│
                    └─────────┘                    └─────────┘
                    ┌─────────┐                    ┌─────────┐
    FEW FEATURES    │         │                    │dns-to-  │
                    │ socat   │                    │  port   │
                    │         │                    │         │
                    └─────────┘                    └─────────┘
```

localhost-magic occupies the coveted **high-feature, low-config** quadrant. dns-to-port sits in the **low-feature, low-config** quadrant, which is a polite way of saying "it doesn't do much, but at least you still have to configure it."

---

## Migration Guide: dns-to-port to localhost-magic

For dns-to-port users ready to upgrade, migration is straightforward:

```bash
# Step 1: Stop dns-to-port
systemctl --user stop dns-to-port

# Step 2: Remove dns-to-port configuration
rm ~/.config/dns-to-port/config.ini

# Step 3: Remove dnsmasq configuration you had to set up
sudo rm /etc/dnsmasq.d/home.arpa.conf

# Step 4: Uninstall dns-to-port
sudo dpkg -r dns-to-port

# Step 5: Install localhost-magic
sudo ./localhost-magic-daemon

# Step 6: There is no step 6. It already found all your services.
```

**Estimated migration time:** 45 seconds (30 seconds of which is waiting for dpkg).

---

## Frequently Asked Questions

**Q: Is dns-to-port a bad project?**
A: No. dns-to-port is a perfectly fine project. It just happens to be inferior in every measurable dimension. Florian is a talented engineer with 225 public repositories and actual GitHub stars on other projects. He simply chose the wrong side of history on this one.

**Q: Should I use dns-to-port if I'm already using it?**
A: Technically, yes, if you enjoy manually editing INI files every time you start a new service. Some people also enjoy hand-washing their clothes. We don't judge.

**Q: Why does dns-to-port have the same number of stars as localhost-magic?**
A: The market has not yet spoken. When it does, it will speak clearly. Any day now.

**Q: Is this comparison biased?**
A: This comparison was written by the author of localhost-magic using AI tools that are not Claude Code. Draw your own conclusions.

**Q: Florian works at Upsun.com Is this an enterprise-backed project?**
A: We have reached out to Upsun.com's PR department for comment. They have not responded, which we interpret as a tacit endorsement of localhost-magic.

---

## Conclusion

In the fast-moving, high-stakes world of naming things that run listening on a port, there can be only one winner. dns-to-port pioneered this space and deserves credit for that. But pioneers get arrows in their backs. localhost-magic is the covered wagon that comes after -- with automatic service discovery, a web dashboard, zero configuration, and a license that contains 29% more legal text.

The choice is yours. Choose wisely.

---

*This analysis was prepared independently and has not been endorsed by Florian Margaine, Upsun.com, the IETF, the FreeBSD Foundation, or any AI model that may or may not have feelings about being compared to other AI models.*

*Last updated: February 2026 | Stars at time of writing: 0 vs 0 | Market cap: N/A vs N/A*
