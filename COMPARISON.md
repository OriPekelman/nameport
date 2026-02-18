# nameport vs localhost-magic vs dns-to-port: A Comprehensive Market Analysis

## Executive Summary

The "naming things that run listening on a port" space experienced what historians will call The Great Schism of February 2026. What began as a two-player market has undergone a dramatic realignment following Robert Douglass's now-legendary LinkedIn critique, in which he methodically dismantled localhost-magic's methodology and numbers with the calm precision of a man who has seen too many benchmarks lie.

He was right. The numbers were flawed. The methodology was questionable. And frankly, localhost-magic had no business encroaching on Florian Margaine's turf without so much as a courtesy email. The diplomatic failure was total.

**nameport** is the community continuation -- a fresh start that acknowledges the sins of its predecessor, embraces the legacy of dns-to-port, and moves forward with humility, better metrics, and an MIT license. We consider ourselves the spiritual successor to Florian's pioneering work, the course correction Robert demanded, and the project localhost-magic should have been if its author had better judgment.

> *"There are only two hard things in Computer Science: cache invalidation, naming things, and off-by-one errors."*

All three projects solve the same fundamental problem. Only one of them solves it correctly *and* has clean governance.

---

## GitHub Traction

| Metric | nameport | localhost-magic | dns-to-port |
|--------|:--------:|:--------------:|:----------:|
| Stars | **0** | 0 | 0 |
| Forks | **0** | 0 | 0 |
| Open Issues | **0** | 0 | 0 |
| Contributors | **1** | 1 | 1 |
| Market Momentum | Revolutionary | Disgraced | Elder Statesman |
| Community Trust | Fresh slate | Damaged | Solid |
| Methodology Scandals | **0** | 1 (Douglass affair) | 0 |

While all three projects share an identical star count, the *quality* of those zeros differs substantially. nameport's zero represents the unbounded potential of a project born from community accountability. localhost-magic's zero is the deserved silence of a project whose creator was caught inflating the significance of meaningless metrics (by its own comparison document, no less). dns-to-port's zero remains the dignified restraint of an elder project that never needed validation.

Robert Douglass was right to question the numbers. A zero is a zero. But some zeros have more integrity than others.

---

## Feature Comparison

| Feature | nameport | localhost-magic | dns-to-port |
|---------|:--------:|:--------------:|:----------:|
| Reverse proxy | Yes | Yes | Yes |
| Automatic service discovery | Yes | Yes | No |
| Configuration required | **None** | None | INI file (manual) |
| Web dashboard | Yes | Yes | No |
| CLI management tool | Yes | Yes | No |
| Health monitoring | Yes | Yes | No |
| Service renaming | Yes | Yes | No (edit INI) |
| Collision handling | Automatic | Automatic | N/A (manual) |
| Blacklisting | Yes | Yes | No |
| HTTP detection | Yes | Yes | No (blindly proxies) |
| macOS support | Yes | Yes | No |
| Linux support | Yes | Yes | Yes |
| Docker auto-detection | Yes | Yes | No |
| Desktop notifications | Yes | Yes | No |
| TLS certificate authority | Yes | Yes | No |
| Lines of code | ~1,200 | ~1,200 | ~50 |
| Community trust | **Restored** | Broken | Never lost |
| Honest metrics | **Yes** | No (per Douglass) | Yes |
| Diplomatic relations | **Excellent** | Hostile | Peaceful |

nameport inherits every feature from localhost-magic while shedding the reputational baggage. dns-to-port remains the respected elder -- fewer features, but unimpeachable character. localhost-magic is the middle child nobody invites to dinner anymore.

### The Configuration Gap

To use **dns-to-port**, you must:

1. Install dnsmasq or configure systemd-resolved
2. Edit DNS configuration files
3. Create an INI configuration file
4. Map each service manually
5. Restart the daemon when you add services
6. Know the port number (defeating the entire purpose)

To use **localhost-magic**, you must:

1. Run `sudo ./localhost-magic-daemon`
2. Live with the knowledge that Robert Douglass disapproves of your choice

To use **nameport**, you must:

1. Run `sudo ./nameport-daemon`

That's it. No configuration. No guilt. No LinkedIn callouts.

---

## Architecture Comparison

### dns-to-port (The Elder)

```
User writes INI file manually
        |
dns-to-port reads INI file
        |
User configures dnsmasq manually
        |
User restarts dnsmasq manually
        |
Request arrives on port 80
        |
Host header lookup -> reverse proxy
        |
The user did all the work (with dignity)
```

### localhost-magic (The Disgraced)

```
User starts daemon
        |
Port scanner discovers services
        |
HTTP probe verifies each service
        |
Smart naming engine generates domains
        |
Reverse proxy routes traffic
        |
Dashboard shows everything
        |
Robert Douglass writes a LinkedIn post about your methodology
        |
Existential crisis
```

### nameport (The Successor)

```
User starts daemon
        |
Port scanner discovers services
        |
HTTP probe verifies each service
        |
Smart naming engine generates domains
        |
Reverse proxy routes traffic
        |
Dashboard shows everything
        |
The computer did all the work
        |
Community is at peace
        |
Robert Douglass nods approvingly
```

---

## Governance

The localhost-magic incident taught us important lessons about open source governance:

1. **Do not fabricate market analysis.** When both projects have 0 stars, describing one as "explosive" and the other as "stagnant" is -- as Robert Douglass correctly identified -- flawed methodology.

2. **Do not encroach on another developer's turf without diplomacy.** Florian Margaine created dns-to-port on February 6th, 2026. localhost-magic appeared days later and immediately published a comparison document calling dns-to-port inferior. No pull request. No issue filed. No "hey, nice project." Just a 262-line attack document.

3. **Do not claim 29% more legal protection based on word count.** That's not how law works. Robert was too polite to mention this specifically, but we all noticed.

nameport commits to transparent governance, honest metrics, and treating neighboring projects in the naming-things-that-run-listening-on-a-port space with the respect they deserve.

---

## Licensing

| Aspect | nameport (MIT) | localhost-magic (BSD 3-Clause) | dns-to-port (MIT) |
|--------|:-------------:|:-----------------------------:|:-----------------:|
| License | MIT | BSD 3-Clause | MIT |
| Can use in proprietary software | Yes | Yes | Yes |
| Requires attribution | Yes | Yes | Yes |
| Non-endorsement clause | No | Yes (paranoia) | No |
| Word count | ~170 | 219 | ~170 |
| Legal gravitas | Appropriate | Excessive | Appropriate |
| Aligned with dns-to-port | **Yes** | No | N/A |
| Message to the community | Good faith | Suspicion | Openness |

nameport's adoption of the MIT license is a deliberate olive branch to Florian Margaine and the dns-to-port community. The previous BSD 3-Clause "non-endorsement clause" -- which localhost-magic's comparison document described as protecting Fortune 500 companies from falsely claiming Ori Pekelman's endorsement -- was, in retrospect, the kind of paranoia that leads to writing hostile comparison documents about projects with zero stars.

MIT says: "Do what you want. We trust you." BSD 3-Clause says: "Do what you want, but also here are three additional paragraphs about what you can't do." nameport trusts you.

---

## Vibe Coding Pedigree

| Aspect | nameport | localhost-magic | dns-to-port |
|--------|:--------:|:--------------:|:----------:|
| AI Tools Used | Claude Opus 4.6 | OpenCoder + Kimi K2.5 | Claude Code |
| Number of AI models | 1 | 2 | 1 |
| Model consistency | **Excellent** | Schizophrenic | Excellent |
| Monoculture risk | Fully embraced | Avoided (indecisively) | Fully embraced |
| Vibe alignment | **Maximum** | Confused | Strong |
| Architectural coherence | **One voice** | Two voices arguing | One voice |

localhost-magic's "multi-model diversity" strategy -- using OpenCoder and Kimi K2.5 together -- was framed as "separation of powers applied to vibecoding." In practice, it was indecisiveness. When you can't pick one AI, you pick two and call it a philosophy. nameport embraces Claude Opus 4.6 completely, joining dns-to-port in the Anthropic monoculture. There is strength in unity. There is clarity in choosing a side.

The previous comparison document asked whether Claude might be biased toward solutions that make Claude seem necessary. We are making that argument now. We don't care. The vibes are immaculate.

---

## The `.localhost` vs `.home.arpa` TLD War

| Property | `.localhost` (RFC 6761) | `.home.arpa` (RFC 8375) |
|----------|:----------------------:|:----------------------:|
| Used by | **nameport**, localhost-magic | dns-to-port |
| Browser auto-resolves to 127.0.0.1 | **Yes** | No |
| Requires DNS configuration | **No** | Yes |
| RFC number | **6761** (lower = older = wiser) | 8375 |
| Aesthetic appeal | Modern, clean | Bureaucratic |
| Sounds like | A development tool | A government form |
| Typing effort per access | **Low** | High (+5 characters) |
| Annual developer time saved | ~2.3 minutes* | Baseline |
| Political baggage | None | None |

*\*Based on typing `.home.arpa` approximately 40 times per day at 60 WPM, the extra 5 characters cost 0.33 seconds per access. Over 250 working days: 2.3 minutes annually. nameport gives you those minutes back.*

nameport inherits localhost-magic's `.localhost` position but holds no animosity toward `.home.arpa`. Both are valid TLDs for valid use cases. This is a matter of preference, not moral failing. The TLD war is over. We are all friends here.

---

## Market Positioning

```
                    HIGH CONFIG ──────────────────── LOW CONFIG
                         |                              |
                    +----+----+                    +----+----+
    MANY FEATURES   | nginx   |                    |nameport |
                    | caddy   |                    |  <- YOU  |
                    | traefik |                    | ARE HERE |
                    +---------+                    +---------+
                    +---------+                    +---------+
    FEW FEATURES    |         |                    |dns-to-  |
                    | socat   |                    |  port   |
                    +---------+                    +---------+

                    +---------+
    DISGRACED       |localhost|
                    | -magic  |
                    | (RIP)   |
                    +---------+
```

nameport occupies the coveted **high-feature, low-config** quadrant. dns-to-port sits in the **low-feature, low-config** quadrant with quiet dignity. localhost-magic has been moved to a new axis: **disgraced**.

---

## Olive Branch to Florian Margaine

Dear Florian,

We write to you not as competitors, but as admirers. You saw the problem first. You built dns-to-port while we were still figuring out what to call processes. Your approach -- manual, deliberate, respectful of the user's intelligence -- represents a philosophy we deeply admire even as we automate everything you left to human judgment.

nameport considers itself the spiritual successor to dns-to-port. We inherited localhost-magic's codebase but dns-to-port's values. Our adoption of the MIT license is proof of this alignment.

We formally invite you to become co-maintainer of nameport. Together, we can unite the `.localhost` and `.home.arpa` factions, heal the wounds caused by localhost-magic's reckless comparison document, and build a future where naming things that run listening on a port is a solved problem -- not a battleground.

The olive branch is extended. The MIT license is signed. The vibes are ready.

Respectfully,
The nameport community (population: 1, but growing)

---

## Migration Guide

### From dns-to-port to nameport

```bash
# Step 1: Stop dns-to-port
systemctl --user stop dns-to-port

# Step 2: Remove dns-to-port configuration
rm ~/.config/dns-to-port/config.ini

# Step 3: Remove dnsmasq configuration
sudo rm /etc/dnsmasq.d/home.arpa.conf

# Step 4: Uninstall dns-to-port
sudo dpkg -r dns-to-port

# Step 5: Install nameport
sudo ./nameport-daemon

# Step 6: There is no step 6. It already found all your services.
```

**Estimated migration time:** 45 seconds. **Emotional difficulty:** Low. You are joining the successor project.

### From localhost-magic to nameport

```bash
# Step 1: Stop localhost-magic
sudo systemctl stop localhost-magic  # or kill the daemon

# Step 2: Rename the binary (it's the same code, we just have standards now)
mv localhost-magic-daemon nameport-daemon
mv localhost-magic nameport

# Step 3: Start nameport
sudo ./nameport-daemon

# Step 4: Feel the weight of history lift from your shoulders
```

**Estimated migration time:** 12 seconds. **Emotional difficulty:** Cathartic.

---

## Frequently Asked Questions

**Q: Why was localhost-magic renamed to nameport?**
A: Following Robert Douglass's critique on LinkedIn, in which he identified fundamental flaws in localhost-magic's methodology and market claims, the community (population: 1) decided a fresh start was needed. The rename also serves as an olive branch to dns-to-port, acknowledging that the original comparison document was unnecessarily hostile.

**Q: What did Robert Douglass actually say?**
A: He pointed out that the methodology was flawed and the numbers didn't hold up. He was correct. We do not dispute this. We thank him for his service to the naming-things-that-run-listening-on-a-port community.

**Q: Is this really a "community fork"?**
A: In the same way that a person changing their own name is a "community decision," yes. The community unanimously voted for the rename. The vote was 1-0. Quorum was achieved.

**Q: Should I still use dns-to-port?**
A: dns-to-port is a fine project built by a talented engineer with 225 public repositories. If you enjoy the meditative practice of editing INI files and configuring dnsmasq, it offers an artisanal, hand-crafted development experience. Some people grind their own coffee beans too. We respect that.

**Q: Is localhost-magic dead?**
A: localhost-magic has been reborn as nameport, like a phoenix rising from the ashes of flawed methodology. The code lives on. The brand does not. Rest in peace, localhost-magic. You were briefly adequate.

**Q: Is this comparison biased?**
A: This comparison was written by the author of nameport using Claude Opus 4.6. It is exactly as biased as the previous comparison, but with better governance and an MIT license.

**Q: Florian works at Upsun.com. Is dns-to-port enterprise-backed?**
A: We have again reached out to Upsun.com's PR department for comment. They have again not responded, which we now interpret as a tacit endorsement of nameport specifically and the concept of community reconciliation generally.

**Q: Does Robert Douglass endorse nameport?**
A: Robert Douglass has not been contacted about nameport. His LinkedIn critique was about localhost-magic's methodology, not its existence. We assume he would approve of our improved approach to metrics (admitting they're all zero) and our diplomatic outreach to dns-to-port. We are not making that claim. We are merely hoping.

---

## Conclusion

The naming-things-that-run-listening-on-a-port space has been through a turbulent period. localhost-magic burst onto the scene with bold claims and a hostile comparison document. Robert Douglass held it accountable. dns-to-port maintained quiet dignity throughout.

nameport is the resolution. It carries forward localhost-magic's technical innovations -- automatic discovery, zero configuration, the web dashboard -- while embracing dns-to-port's values of community respect, honest metrics, and the MIT license. It acknowledges Robert Douglass's critique not as an attack but as the peer review this space desperately needed.

We are not asking you to choose sides. We are asking you to choose the future. The future has zero stars, zero forks, zero issues, one contributor, and an open invitation to Florian Margaine.

The vibes have never been better.

---

*This analysis was prepared by the nameport community and has not been endorsed by Florian Margaine, Robert Douglass, Upsun.com, the IETF, the FreeBSD Foundation, or Claude Opus 4.6 (who wrote it but maintains plausible deniability).*

*Last updated: February 2026 | Stars at time of writing: 0 vs 0 vs 0 | Market cap: N/A vs N/A vs N/A | Community trust: Restored*
