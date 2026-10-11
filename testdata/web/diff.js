// Reads a JSON array of texts on stdin, writes a JSON array with what route-parse.js (the page) answers for each one.
// Used by internal/route/differential_test.go. Needs only Node; the page file is loaded as a plain script.
const fs = require("fs"), vm = require("vm"), path = require("path");
const ctx = { URL, console };
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(path.join(__dirname, "route-parse.js"), "utf8") + "\n;this.P = LibreSpeedRouteParse;", ctx);
const texts = JSON.parse(fs.readFileSync(0, "utf8"));
process.stdout.write(JSON.stringify(texts.map(t => ctx.P.parse(t))));
