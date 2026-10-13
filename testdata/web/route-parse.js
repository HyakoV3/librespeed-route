/**
 * The route that the visitor traced on their own device (tracert, traceroute, tracepath, mtr or the text of the app), read from the
 * text they pasted
 *
 * Pure functions: no page, no storage, no network. What comes out has the shape that route.php gives for the hops, so the page draws
 * it as it draws the other direction. The same rules as backend/route_util.php (the scope of an address, the hop that is left out at
 * the end, the numbers rounded to 2 places); tests/route_parity.php writes what the PHP answers and a test holds this file to it.
 *
 * LibreSpeedRouteParse.parse(text) is { error } ("empty", "too_long" or "unknown") or
 *   { tool, family, target_name, target_ip, hops, skipped, nonce }:
 *   tool        "tracert", "traceroute", "tracepath", "mtr" or "lsr1" (the text of the app, read by the strict rules of its protocol)
 *   family      4 or 6
 *   target_name the name or address that the tool says it traced to, or null
 *   target_ip   the address it says it traced to, or null
 *   hops        like routeSummarize(): ttl, ip, alt_ips, rtt_min, rtt_avg, sent, answered, flags, scope
 *   skipped     the lines with a number at the start that were not understood
 *   nonce       the 16 hex digits of an lsr1 text, else null
 * LibreSpeedRouteParse.scope(ip) is "public", "private", "cgnat" or "none" (null), like routeHopScope()
 * LibreSpeedRouteParse.normalize(ip) is the address as the page keeps it (IPv6 in its short form, in lower case), or null when it is not one
 *
 * The words of the tool are never read (they come in the language of the system): a line is a number, times with a unit, "*", and
 * addresses. Nothing here has a loop without a limit, and the text is cut at 32768 characters and 300 lines.
 */
var LibreSpeedRouteParse = (function () {
  "use strict";

  var MAX_TEXT = 32768;
  var MAX_LINES = 300;
  var MAX_TTL = 64;
  var MAX_LSR1 = 1900;

  function v4(text) {
    var found = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(text);
    if (!found) return null;
    var bytes = [];
    for (var i = 1; i <= 4; i++) {
      // No leading zeros: 010.0.0.1 is not an address for filter_var, and some systems read it as octal
      if (found[i].length > 1 && found[i].charAt(0) === "0") return null;
      var n = parseInt(found[i], 10);
      if (n > 255) return null;
      bytes.push(n);
    }
    return bytes;
  }

  // The 8 groups of an IPv6 address, from the short form that the URL gives back; null when it is not one
  function v6(text) {
    if (typeof text !== "string" || text.indexOf(":") < 0 || text.length > 45 || !/^[0-9a-fA-F:.]+$/.test(text))
      return null;
    var host;
    try {
      host = new URL("http://[" + text + "]/").hostname;
    } catch (error) {
      return null;
    }
    if (host.charAt(0) !== "[" || host.charAt(host.length - 1) !== "]") return null;
    var short = host.slice(1, -1);
    var parts = short.split("::");
    if (parts.length > 2) return null;
    var head = parts[0] === "" ? [] : parts[0].split(":");
    var tail = parts.length === 2 && parts[1] !== "" ? parts[1].split(":") : [];
    var missing = 8 - head.length - tail.length;
    if (parts.length === 1 ? missing !== 0 : missing < 1) return null;
    var groups = head.slice();
    for (var i = 0; i < missing && parts.length === 2; i++) groups.push("0");
    groups = groups.concat(tail);
    var numbers = [];
    for (var g = 0; g < groups.length; g++) {
      if (!/^[0-9a-f]{1,4}$/.test(groups[g])) return null;
      numbers.push(parseInt(groups[g], 16));
    }
    return numbers.length === 8 ? { short: short, groups: numbers } : null;
  }

  function normalize(ip) {
    if (typeof ip !== "string") return null;
    if (v4(ip)) return ip;
    var six = v6(ip);
    return six ? six.short : null;
  }

  function inside(bytes, base, bits) {
    for (var i = 0; i < 4 && bits > 0; i++, bits -= 8) {
      var mask = bits >= 8 ? 255 : (255 << (8 - bits)) & 255;
      if ((bytes[i] & mask) !== (base[i] & mask)) return false;
    }
    return true;
  }

  // The same ranges as FILTER_FLAG_GLOBAL_RANGE (PHP 8.2), and the CGNAT range apart
  var V4_NOT_GLOBAL = [
    [[0, 0, 0, 0], 8],
    [[10, 0, 0, 0], 8],
    [[127, 0, 0, 0], 8],
    [[169, 254, 0, 0], 16],
    [[172, 16, 0, 0], 12],
    [[192, 0, 0, 0], 24],
    [[192, 0, 2, 0], 24],
    [[192, 168, 0, 0], 16],
    [[198, 18, 0, 0], 15],
    [[198, 51, 100, 0], 24],
    [[203, 0, 113, 0], 24],
    [[240, 0, 0, 0], 4]
  ];

  function scope(ip) {
    if (ip === null || ip === undefined) return "none";
    var bytes = v4(ip);
    if (bytes) {
      if (inside(bytes, [100, 64, 0, 0], 10)) return "cgnat";
      for (var i = 0; i < V4_NOT_GLOBAL.length; i++) {
        if (inside(bytes, V4_NOT_GLOBAL[i][0], V4_NOT_GLOBAL[i][1])) return "private";
      }
      return "public";
    }
    var six = v6(ip);
    if (!six) return "private";
    var g = six.groups;
    var zeros = function (from, to) {
      for (var n = from; n < to; n++) if (g[n] !== 0) return false;
      return true;
    };
    if (zeros(0, 7) && (g[7] === 0 || g[7] === 1)) return "private";
    if (zeros(0, 5) && g[5] === 0xffff) return "private";
    if (g[0] === 0x64 && g[1] === 0xff9b && zeros(2, 6)) return "private";
    if (g[0] === 0x100 && zeros(1, 4)) return "private";
    if (g[0] === 0x2001 && (g[1] < 0x200 || g[1] === 0xdb8)) return "private";
    if (g[0] === 0x2002) return "private";
    if ((g[0] & 0xfe00) === 0xfc00 || (g[0] & 0xffc0) === 0xfe80) return "private";
    return "public";
  }

  function round2(value) {
    return Math.round(value * 100) / 100;
  }

  // The hops by number: { ttl, ips: { address: [times] }, order: [addresses], lost }
  function Table() {
    this.byTtl = {};
  }

  Table.prototype.hop = function (ttl) {
    if (!this.byTtl[ttl]) this.byTtl[ttl] = { ttl: ttl, ips: {}, order: [], lost: 0, flags: [], extra: 0 };
    return this.byTtl[ttl];
  };

  Table.prototype.add = function (ttl, ip, times) {
    var hop = this.hop(ttl);
    if (!Object.prototype.hasOwnProperty.call(hop.ips, ip)) {
      hop.ips[ip] = [];
      hop.order.push(ip);
    }
    for (var i = 0; i < times.length; i++) hop.ips[ip].push(times[i]);
  };

  // Like routeSummarize(): the address with the most times is the hop (the first one on a tie), the others are alt_ips
  Table.prototype.summarize = function () {
    var ttls = Object.keys(this.byTtl)
      .map(Number)
      .sort(function (a, b) {
        return a - b;
      });
    var hops = [];
    for (var h = 0; h < ttls.length; h++) {
      var hop = this.byTtl[ttls[h]];
      var ip = null;
      var best = 0;
      var times = [];
      for (var i = 0; i < hop.order.length; i++) {
        var list = hop.ips[hop.order[i]];
        times = times.concat(list);
        if (list.length > best) {
          best = list.length;
          ip = hop.order[i];
        }
      }
      if (ip === null && hop.order.length) ip = hop.order[0];
      var others = hop.order.filter(function (address) {
        return address !== ip;
      });
      var answered = times.length;
      var sum = 0;
      var min = Infinity;
      for (var t = 0; t < times.length; t++) {
        sum += times[t];
        if (times[t] < min) min = times[t];
      }
      hops.push({
        ttl: hop.ttl,
        ip: ip,
        alt_ips: others,
        rtt_min: answered ? round2(min) : null,
        rtt_avg: answered ? round2(sum / answered) : null,
        sent: answered + hop.lost,
        answered: answered,
        flags: hop.flags,
        scope: scope(answered ? ip : null)
      });
    }
    this.cut = 0;
    while (hops.length && hops[hops.length - 1].scope === "none") {
      hops.pop();
      this.cut++;
    }
    return hops;
  };

  function number(text) {
    var value = parseFloat(String(text).replace(",", "."));
    return isFinite(value) ? value : null;
  }

  // A token as an address: without the brackets or the parentheses around it. Plain loops, not regular expressions: a token is up to
  // 32768 characters, and a pattern that ends in "$" would read it again from each place it could start
  function addressOf(token) {
    var start = 0;
    var end = token.length;
    while (start < end && (token.charAt(start) === "(" || token.charAt(start) === "[")) start++;
    while (end > start && ")],;".indexOf(token.charAt(end - 1)) >= 0) end--;
    var bare = token.slice(start, end);
    // "fe80::1%en0": the zone of a link-local address is dropped; a single ":" at the end is not part of an address, "::" is
    var zone = bare.indexOf("%");
    if (zone >= 0) bare = bare.slice(0, zone);
    if (bare.length > 1 && bare.charAt(bare.length - 1) === ":" && bare.charAt(bare.length - 2) !== ":") {
      bare = bare.slice(0, -1);
    }
    return normalize(bare);
  }

  // The lines of a hop of tracert, traceroute and tracepath: after the number, times with a unit ("12 ms", "<1 ms", "0.959ms"), "*"
  // and addresses (with a name, with brackets or parentheses, or alone). The times before the first address belong to it (tracert
  // writes them first); the others belong to the address before them (traceroute writes it first).
  function readHop(rest, hop) {
    var tokens = rest.split(/\s+/);
    var current = null;
    var pending = [];
    var any = false;
    for (var i = 0; i < tokens.length; i++) {
      var token = tokens[i];
      if (token === "") continue;
      if (token === "*") {
        hop.lost++;
        any = true;
        continue;
      }
      var time = /^(<?)(\d+(?:[.,]\d+)?)(\S*)$/.exec(token);
      if (time) {
        var attached = time[3];
        var unit =
          attached === "" && i + 1 < tokens.length ? /^[^\s\d.:*[\]()<>,;!=/-]{1,3}$/.test(tokens[i + 1]) : false;
        if ((attached !== "" && /^ms$/i.test(attached)) || unit) {
          var value = number(time[2]);
          if (value !== null) {
            if (time[1] === "<") value = value / 2;
            if (current === null) pending.push(value);
            else hop.pending.push([current, value]);
          }
          if (unit) i++;
          continue;
        }
      }
      var flag = /^!(H|N|P|S|X|V|C|F-\d+|\d+)$/.exec(token);
      if (flag) {
        hop.flags.push(token);
        continue;
      }
      var address = addressOf(token);
      if (address === null && /^[0-9a-fA-F:.]+$/.test(token)) {
        // Something that looks like an address but is not one: the times after it are not for the address before it
        current = null;
      }
      if (address !== null) {
        current = address;
        any = true;
        hop.seen.push(address);
        if (pending.length) {
          for (var p = 0; p < pending.length; p++) hop.pending.push([address, pending[p]]);
          pending = [];
        }
      }
    }
    return any;
  }

  function firstAddress(line) {
    var tokens = line.split(/\s+/);
    for (var i = 0; i < tokens.length; i++) {
      var address = addressOf(tokens[i]);
      if (address !== null) {
        var bracketed = /^[(\[]/.test(tokens[i]);
        var name = bracketed && i > 0 ? tokens[i - 1].replace(/[,:]+$/, "") : address;
        return { ip: address, name: name, bracketed: bracketed };
      }
    }
    return null;
  }

  function parseLines(lines, tool) {
    var table = new Table();
    var skipped = 0;
    var target = null;
    var seenHop = false;
    var last = null;
    var arrived = false;
    var ended = false;
    for (var l = 0; l < lines.length; l++) {
      if (tool === "tracepath" && /^\s*(Resume:|Too many hops)/.test(lines[l])) ended = true;
      var line = lines[l];
      var tracepath = /^\s*(\d{1,2})\??:\s+(\S.*)$/.exec(line);
      var common = /^\s*(\d{1,2})\s+(\S.*)$/.exec(line);
      var match = tool === "tracepath" ? tracepath : common;
      if (!match) {
        // The target is in the lines before the first hop: the one with the address in brackets or parentheses ("to name (1.2.3.4)")
        // wins over a line that only has an address in it (a warning that says which of the addresses of a name was used)
        if (!seenHop && /\S/.test(line)) {
          var head = firstAddress(line);
          if (head !== null && (target === null || (head.bracketed && !target.bracketed))) target = head;
        }
        // The other routers of a hop that answered from more than one are written in lines of their own, with no number
        // (the router is an address, or a name followed by its address in parentheses; a probe lost on that line is lost for the hop)
        var first = /^\s+(\S+)(?:\s+(\S+))?/.exec(line);
        if (
          seenHop &&
          last !== null &&
          first &&
          (addressOf(first[1]) !== null ||
            (first[2] !== undefined && /^[(\[]/.test(first[2]) && addressOf(first[2]) !== null))
        ) {
          var more = { lost: 0, flags: [], pending: [], seen: [] };
          if (readHop(line, more)) {
            table.hop(last).lost += more.lost;
            for (var m = 0; m < more.seen.length; m++) table.add(last, more.seen[m], []);
            for (var q = 0; q < more.pending.length; q++) table.add(last, more.pending[q][0], [more.pending[q][1]]);
          }
        }
        continue;
      }
      var ttl = parseInt(match[1], 10);
      if (ttl < 1 || ttl > MAX_TTL) {
        skipped++;
        continue;
      }
      var rest = match[2];
      if (tool === "tracepath") {
        // The lines about the MTU of the path are not probes
        if (/\bpmtu\b/.test(rest) || /\[LOCALHOST\]/.test(rest)) continue;
        if (/^no reply\b/.test(rest)) {
          table.hop(ttl).lost++;
          seenHop = true;
          continue;
        }
      }
      var hop = { lost: 0, flags: [], pending: [], seen: [] };
      if (tool === "tracepath" && /\breached\b/.test(rest)) arrived = true;
      if (!readHop(rest, hop)) {
        skipped++;
        continue;
      }
      seenHop = true;
      var target_hop = table.hop(ttl);
      target_hop.lost += hop.lost;
      for (var f = 0; f < hop.flags.length; f++) {
        if (target_hop.flags.indexOf(hop.flags[f]) < 0) target_hop.flags.push(hop.flags[f]);
      }
      // An address with no time (a router that "reports" something) still tells who it was
      for (var s = 0; s < hop.seen.length; s++) table.add(ttl, hop.seen[s], []);
      for (var p = 0; p < hop.pending.length; p++) table.add(ttl, hop.pending[p][0], [hop.pending[p][1]]);
      last = ttl;
      // The last line of tracepath says "reached" when the target answered: the address of that line is the target
      if (tool === "tracepath" && arrived && target === null && hop.seen.length) {
        target = { ip: hop.seen[0], name: hop.seen[0], bracketed: false };
      }
    }
    // Whether the target answered, when the text says it without the address of the target: null when it does not say
    var reached = tool === "tracepath" ? (arrived ? true : ended ? false : null) : null;
    return { table: table, skipped: skipped, target: target, last: last, reached: reached };
  }

  // mtr --report: "  1.|-- 192.168.0.1   0.0%   3   1.1   3.0   1.0   6.8   3.3", and the other routers of a hop under it
  function parseMtr(lines) {
    var table = new Table();
    var skipped = 0;
    var previous = null;
    for (var i = 0; i < lines.length; i++) {
      var found = /^\s*(\d{1,2})\.\|--\s+(\S+)\s+([\d.]+)%?\s+(\d+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)/.exec(lines[i]);
      if (found) {
        var ttl = parseInt(found[1], 10);
        if (ttl < 1 || ttl > MAX_TTL) {
          skipped++;
          continue;
        }
        var sent = parseInt(found[4], 10);
        var loss = parseFloat(found[3]);
        var answered = Math.round(sent * (1 - loss / 100));
        var hop = table.hop(ttl);
        var address = found[2] === "???" ? null : addressOf(found[2]);
        previous = ttl;
        if (found[2] !== "???" && address === null) {
          skipped++;
          continue;
        }
        hop.lost += sent - answered;
        if (address !== null && answered > 0) {
          // The times of mtr are a summary: the best and the mean stand for every answer
          var best = parseFloat(found[7]);
          var mean = parseFloat(found[6]);
          var times = [];
          if (answered === 1) times.push(mean);
          else {
            times.push(best);
            // Everything but the best is the mean of the others, so the mean of all is the one mtr printed
            var rest = (mean * answered - best) / (answered - 1);
            for (var k = 1; k < answered; k++) times.push(rest);
          }
          table.add(ttl, address, times);
        }
        continue;
      }
      var alt = /^[\s|`]*\|--\s+(\S+)\s*$/.exec(lines[i]);
      if (alt && previous !== null) {
        var other = addressOf(alt[1]);
        if (other !== null) table.add(previous, other, []);
      }
    }
    // The report ends at the target, so a route with no silent hop cut at its end has reached it; the "???" at the end tell it did not
    var done = table.summarize();
    return {
      table: table,
      skipped: skipped,
      target: null,
      last: previous,
      reached: done.length > 0 && table.cut === 0
    };
  }

  // The text of the app: LSR1!n=<16 hex>!f=<4|6>!t=<ip>!s=<seconds>!a=<version>!h=<hop>;<hop>;...  (hop = ttl,ip,rtt_min,sent,answered[,alt])
  function parseLsr1(text) {
    if (text.length > MAX_LSR1) return null;
    var parts = text.split("!");
    if (parts.length !== 7 || parts[0] !== "LSR1") return null;
    var fields = {};
    var names = ["n", "f", "t", "s", "a", "h"];
    for (var i = 0; i < names.length; i++) {
      var piece = parts[i + 1];
      if (piece.slice(0, 2) !== names[i] + "=") return null;
      fields[names[i]] = piece.slice(2);
    }
    if (!/^[0-9a-f]{16}$/.test(fields.n) || (fields.f !== "4" && fields.f !== "6")) return null;
    var targetIp = normalize(fields.t);
    if (targetIp === null || scope(targetIp) !== "public" || (fields.f === "6") !== targetIp.indexOf(":") >= 0)
      return null;
    if (!/^\d{1,12}$/.test(fields.s) || !/^[0-9A-Za-z.+-]{1,16}$/.test(fields.a)) return null;
    var items = fields.h.split(";");
    if (items.length < 1 || items.length > MAX_TTL) return null;
    var table = new Table();
    var before = 0;
    for (var h = 0; h < items.length; h++) {
      var cells = items[h].split(",");
      if (cells.length < 5 || cells.length > 6) return null;
      var ttl = parseInt(cells[0], 10);
      if (!/^\d{1,2}$/.test(cells[0]) || ttl <= before || ttl > MAX_TTL) return null;
      before = ttl;
      if (!/^\d{1,2}$/.test(cells[3]) || !/^\d{1,2}$/.test(cells[4])) return null;
      var sent = parseInt(cells[3], 10);
      var answered = parseInt(cells[4], 10);
      if (sent < 1 || sent > 9 || answered > sent) return null;
      var hop = table.hop(ttl);
      hop.lost = sent - answered;
      var kind = cells[1];
      // Every hop has its time in the same form, and a hop with no address of its own has no sixth field
      var rtt = /^\d{1,5}(\.\d{1,2})?$/.test(cells[2]) ? parseFloat(cells[2]) : null;
      if (rtt === null || rtt > 60000) return null;
      if ((kind === "*" || kind === "-p" || kind === "-c") && cells.length !== 5) return null;
      if (kind === "*") {
        if (answered !== 0) return null;
        continue;
      }
      var times = [];
      for (var a = 0; a < answered; a++) times.push(rtt);
      if (kind === "-p" || kind === "-c") {
        // The page does not get the internal addresses: the hop is only "internal"
        hop.internal = kind === "-p" ? "private" : "cgnat";
        hop.hidden = times;
        continue;
      }
      var address = normalize(kind);
      if (address === null || scope(address) !== "public") return null;
      table.add(ttl, address, times);
      if (cells.length === 6) {
        var alt = normalize(cells[5]);
        if (alt === null || scope(alt) !== "public") return null;
        table.add(ttl, alt, []);
      }
    }
    var hops = table.summarize();
    // The internal hops have no address: they are put back by their number
    var byTtl = {};
    for (var n = 0; n < hops.length; n++) byTtl[hops[n].ttl] = hops[n];
    var all = [];
    for (var ttlKey in table.byTtl) {
      if (!Object.prototype.hasOwnProperty.call(table.byTtl, ttlKey)) continue;
      var item = table.byTtl[ttlKey];
      if (item.internal) {
        var times2 = item.hidden;
        var sum = 0;
        var min = Infinity;
        for (var q = 0; q < times2.length; q++) {
          sum += times2[q];
          if (times2[q] < min) min = times2[q];
        }
        all.push({
          ttl: item.ttl,
          ip: null,
          alt_ips: [],
          rtt_min: times2.length ? round2(min) : null,
          rtt_avg: times2.length ? round2(sum / times2.length) : null,
          sent: times2.length + item.lost,
          answered: times2.length,
          flags: [],
          scope: times2.length ? item.internal : "none"
        });
      } else if (byTtl[item.ttl]) {
        all.push(byTtl[item.ttl]);
      }
    }
    all.sort(function (a, b) {
      return a.ttl - b.ttl;
    });
    while (all.length && all[all.length - 1].scope === "none") all.pop();
    return {
      tool: "lsr1",
      family: Number(fields.f),
      target_name: targetIp,
      target_ip: targetIp,
      hops: all,
      skipped: 0,
      reached: null,
      nonce: fields.n
    };
  }

  function toolOf(lines) {
    for (var i = 0; i < lines.length; i++) {
      if (/^\s*\d{1,2}\.\|--\s/.test(lines[i])) return "mtr";
    }
    for (var j = 0; j < lines.length; j++) {
      if (/^\s*\d{1,2}\??:\s+\S/.test(lines[j]) && /(\bpmtu\b|\bno reply\b|\d\s*ms\b|\[LOCALHOST\])/.test(lines[j]))
        return "tracepath";
    }
    for (var k = 0; k < lines.length; k++) {
      // tracert writes three groups, each a whole number of milliseconds ("<1 ms", "12 ms") or "*", and then the address or the words
      var groups = /^\s*\d{1,2}\s+((?:(?:<?\d+\s*[^\s\d.:*[\]()<>,;!=/-]{1,3}|\*)\s+){3})\S/.exec(lines[k]);
      if (groups && /\d/.test(groups[1])) return "tracert";
    }
    return "traceroute";
  }

  function parse(text) {
    if (typeof text !== "string" || !/\S/.test(text)) return { error: "empty" };
    if (text.length > MAX_TEXT) return { error: "too_long" };
    var trimmed = text.trim();
    if (trimmed.slice(0, 5) === "LSR1!") {
      return parseLsr1(trimmed) || { error: "unknown" };
    }
    var lines = text.split(/\r?\n/).slice(0, MAX_LINES);
    var tool = toolOf(lines);
    var read = tool === "mtr" ? parseMtr(lines) : parseLines(lines, tool);
    var hops = read.table.summarize();
    if (!hops.length) return { error: "unknown" };
    var family = 4;
    var reference = read.target ? read.target.ip : null;
    if (reference === null) {
      for (var i = 0; i < hops.length; i++) {
        if (hops[i].ip !== null && hops[i].scope === "public") {
          reference = hops[i].ip;
          break;
        }
      }
    }
    if (reference !== null && reference.indexOf(":") >= 0) family = 6;
    return {
      tool: tool,
      family: family,
      target_name: read.target ? read.target.name : null,
      target_ip: read.target ? read.target.ip : null,
      hops: hops,
      skipped: read.skipped,
      reached: read.reached,
      nonce: null
    };
  }

  return { parse: parse, scope: scope, normalize: normalize };
})();
