
// mock-server.js
import express from "express";
import morgan from "morgan";
import fs from "fs";
import path from "path";

const app = express();
const PORT = 3000;

// --- Logging setup ---
const logDir = "./logs";
if (!fs.existsSync(logDir)) fs.mkdirSync(logDir);

const accessLogStream = fs.createWriteStream(path.join(logDir, "access.log"), {
	flags: "a",
});

// Log to both console and file
app.use(
	morgan(":date[iso] :method :url :status :res[content-length] - :response-time ms", {
		stream: accessLogStream,
	})
);
app.use(morgan("dev"));

app.use(express.json());

// --- Utility ---
function delay(ms) {
	return new Promise((res) => setTimeout(res, ms));
}

// --- Routes ---

// 🧍 User API
app.get("/api/users", async (req, res) => {
	await delay(100); // simulate DB delay
	res.json({
		message: "User list fetched successfully",
		users: [
			{ id: 1, name: "Alice" },
			{ id: 2, name: "Bob" },
		],
		headers: req.headers,
	});
});

app.get("/api/users/:id", async (req, res) => {
	await delay(80);
	res.json({
		message: "User detail",
		id: req.params.id,
		headers: req.headers,
	});
});

app.post("/api/users", (req, res) => {
	res.status(201).json({
		message: "User created",
		body: req.body,
		headers: req.headers,
	});
});

app.get("/api/users/private", (req, res) => {
	res.status(403).json({ message: "Forbidden: private endpoint" });
});

// 🌍 Public API
app.get("/api/public", (req, res) => {
	res.json({
		message: "Public API endpoint",
		time: new Date().toISOString(),
	});
});

// 💎 Premium API
app.get("/api/premium", (req, res) => {
	res.json({
		message: "Premium API data",
		plan: "gold",
		user: req.headers["x-api-key"] || "unknown",
	});
});

// 🧩 Internal API
app.get("/api/internal", (req, res) => {
	res.json({
		message: "Internal service data",
		uptime: process.uptime(),
	});
});

// // --- Catch-all route ---
// app.all("/*", (req, res) => {
// 	res.status(404).json({ message: "Not Found", path: req.path });
// });

// --- Start Server ---
app.listen(PORT, () => {
	console.log(`[MockServer] Listening on http://localhost:${PORT}`);
});
