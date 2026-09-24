(() => {
  let conn = null,
    myName = "",
    myRoom = "lobby",
    members = [],
    autoScroll = true,
    showTimestamps = true;
  let isMuted = false,
    audioCtx = null,
    clientColor = "#000000",
    clientBold = false,
    clientItalic = false;
  const $ = (id) => document.getElementById(id);
  const stream = $("chat-stream"),
    rows = $("userlist-rows"),
    input = $("chat-input");
  const overlay = $("join-overlay"),
    joinForm = $("join-form"),
    joinErr = $("join-error");
  const esc = (s) =>
    String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  const ts = () => new Date().toLocaleTimeString();
  function tone(f, d, type, g) {
    if (isMuted) return;
    try {
      audioCtx =
        audioCtx || new (window.AudioContext || window.webkitAudioContext)();
      const o = audioCtx.createOscillator(),
        gn = audioCtx.createGain();
      o.type = type || "sine";
      o.frequency.setValueAtTime(f, audioCtx.currentTime);
      gn.gain.setValueAtTime(g || 0.06, audioCtx.currentTime);
      gn.gain.exponentialRampToValueAtTime(0.0001, audioCtx.currentTime + d);
      o.connect(gn);
      gn.connect(audioCtx.destination);
      o.start();
      o.stop(audioCtx.currentTime + d);
    } catch (e) {}
  }
  const sndMsg = () => {
    tone(880, 0.08, "triangle");
    setTimeout(() => tone(1174, 0.12), 60);
  };
  const sndJoin = () => {
    tone(523, 0.06);
    setTimeout(() => tone(659, 0.06), 60);
    setTimeout(() => tone(784, 0.1), 120);
  };
  const sndQuit = () => {
    tone(784, 0.06);
    setTimeout(() => tone(659, 0.06), 60);
    setTimeout(() => tone(523, 0.1), 120);
  };
  window.toggleAudio = () => {
    isMuted = !isMuted;
    $("soundIcon").className = isMuted
      ? "fa-solid fa-volume-xmark text-[#606060]"
      : "fa-solid fa-volume-high text-[#3f6b0f]";
    $("soundLabel").textContent = isMuted ? "Sound: OFF" : "Sound: ON";
    if (!isMuted) sndMsg();
  };
  joinForm.onsubmit = (ev) => {
    ev.preventDefault();
    joinErr.textContent = "";
    const n = $("name-input").value.trim(),
      r = $("room-input").value.trim().toLowerCase() || "lobby";
    if (!n) return;
    dial(n, r);
  };
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") sendUserMessage();
  });
  $("chk-autoscroll").addEventListener(
    "change",
    (e) => (autoScroll = e.target.checked),
  );
  $("chk-timestamps").addEventListener("change", (e) => {
    showTimestamps = e.target.checked;
    document
      .querySelectorAll(".chat-time")
      .forEach((s) => (s.style.display = showTimestamps ? "" : "none"));
  });
  window.setChatColor = (c) => {
    clientColor = c;
    tone(550, 0.03);
  };
  window.toggleBold = () => {
    clientBold = !clientBold;
    $("btn-bold").classList.toggle("pressed", clientBold);
  };
  window.toggleItalic = () => {
    clientItalic = !clientItalic;
    $("btn-italic").classList.toggle("pressed", clientItalic);
  };
  window.toggleEmoticonTray = (e) => {
    if (e) e.stopPropagation();
    $("emoticon-popover").classList.toggle("hidden");
  };
  window.insertEmoji = (em) => {
    input.value += em;
    input.focus();
    $("emoticon-popover").classList.add("hidden");
  };
  document.addEventListener("click", (e) => {
    const t = $("emoticon-popover");
    if (t && !t.contains(e.target)) t.classList.add("hidden");
  });
  window.toggleMobileSidebar = () => {
    const sb = $("userlist-sidebar"),
      bd = $("mobile-backdrop");
    if (sb.classList.contains("hidden")) {
      sb.classList.remove("hidden");
      sb.classList.add(
        "fixed",
        "inset-y-0",
        "right-0",
        "z-40",
        "w-72",
        "shadow-2xl",
      );
      bd.classList.remove("hidden");
    } else {
      sb.classList.add("hidden");
      sb.classList.remove(
        "fixed",
        "inset-y-0",
        "right-0",
        "z-40",
        "w-72",
        "shadow-2xl",
      );
      bd.classList.add("hidden");
    }
  };
  window.filterRoster = (q) => {
    q = q.toLowerCase();
    for (const row of rows.children) {
      row.style.display = row.textContent.toLowerCase().includes(q)
        ? ""
        : "none";
    }
  };
  function setConn(on) {
    $("conn-status").textContent = on ? "connected" : "disconnected";
    $("conn-dot").className =
      "w-2.5 h-2.5 rounded-full border border-white inline-block " +
      (on ? "bg-green-500 blink" : "bg-red-500");
    $("footer-ws").textContent = on ? "Connected" : "Disconnected";
    $("footer-ws").className = on
      ? "text-[#008000] font-bold"
      : "text-[#800000] font-bold";
    $("self-dot").className =
      "w-2.5 h-2.5 rounded-full border border-white inline-block " +
      (on ? "bg-[#00ff00] blink" : "bg-gray-400");
  }
  function dial(name, room) {
    myName = name;
    myRoom = room;
    if (conn) {
      try {
        conn.close(1000, "switching");
      } catch (e) {}
      conn = null;
    }
    const proto = location.protocol === "https:" ? "wss" : "ws";
    conn = new WebSocket(
      proto +
        "://" +
        location.host +
        "/subscribe?name=" +
        encodeURIComponent(name) +
        "&room=" +
        encodeURIComponent(room),
    );
    conn.addEventListener("open", () => {
      overlay.classList.add("hidden");
      setConn(true);
      $("self-name").textContent = name;
      setRoom(room);
      appendSys("*** Connected as " + name + " to #" + room + " ***");
      input.focus();
    });
    conn.addEventListener("message", (ev) => {
      let m;
      try {
        m = JSON.parse(ev.data);
      } catch (e) {
        return;
      }
      onMsg(m);
    });
    conn.addEventListener("close", (ev) => {
      setConn(false);
      if (ev.code === 1000) {
        overlay.classList.remove("hidden");
        appendSys("*** You left #" + myRoom + " ***");
        conn = null;
        return;
      }
      joinErr.textContent = ev.reason || "Connection closed (" + ev.code + ")";
      overlay.classList.remove("hidden");
      appendSys(
        "*** Disconnected (" + ev.code + " " + (ev.reason || "") + ") ***",
      );
      conn = null;
    });
    conn.addEventListener("error", () => {});
  }
  function setRoom(r) {
    myRoom = r;
    ["header-room", "title-room", "footer-room"].forEach(
      (id) => ($(id).textContent = r),
    );
  }
  function onMsg(m) {
    switch (m.type) {
      case "presence":
        members = m.members || [];
        renderRoster();
        appendSys(
          "*** Joined #" + m.room + " (" + members.length + " here) ***",
        );
        sndJoin();
        break;
      case "join":
        appendSys("Join: " + m.from);
        sndJoin();
        refreshRooms();
        break;
      case "leave":
        appendSys("Quit: " + m.from);
        sndQuit();
        refreshRooms();
        break;
      case "chat":
        appendChat(m);
        sndMsg();
        break;
      case "system":
        appendSys("[announce] " + m.text);
        sndMsg();
        break;
      case "error":
        appendErr(m.text || "error");
        break;
      default:
        console.warn("unknown", m);
    }
  }
  function scroll() {
    if (autoScroll) stream.scrollTop = stream.scrollHeight;
  }
  function appendSys(t) {
    const d = document.createElement("div");
    d.className = "text-[#800000] text-[11px]";
    d.textContent = (showTimestamps ? "(" + ts() + ") " : "") + t;
    stream.appendChild(d);
    scroll();
  }
  function appendErr(t) {
    const d = document.createElement("div");
    d.className = "text-[#a00000] text-[11px] font-bold";
    d.textContent = (showTimestamps ? "(" + ts() + ") " : "") + "ERROR: " + t;
    stream.appendChild(d);
    scroll();
  }
  function lineColor(name) {
    let h = 0;
    for (const c of name) h = (h * 31 + c.charCodeAt(0)) >>> 0;
    return ["#000080", "#800080", "#008000", "#c00000", "#004080", "#606000"][
      h % 6
    ];
  }
  function appendChat(m) {
    const d = document.createElement("div");
    d.className = "chat-entry";
    const who = document.createElement("span");
    who.className = "font-bold chat-time";
    who.style.color = m.from === myName ? "#008000" : lineColor(m.from || "?");
    who.textContent =
      (showTimestamps
        ? "(" + new Date(m.time).toLocaleTimeString() + ") "
        : "") +
      (m.from === myName ? "You" : m.from) +
      ": ";
    const body = document.createElement("span");
    body.className = "ml-1";
    body.style.color = m.from === myName ? clientColor : "#000";
    if (m.from === myName) {
      if (clientBold) body.style.fontWeight = "bold";
      if (clientItalic) body.style.fontStyle = "italic";
    }
    body.textContent = m.text || "";
    d.appendChild(who);
    d.appendChild(body);
    stream.appendChild(d);
    scroll();
  }
  function renderRoster() {
    rows.innerHTML = "";
    const sorted = [...members].sort();
    for (const nick of sorted) {
      const row = document.createElement("div");
      row.className =
        "flex items-center justify-between p-0.5 hover:bg-[#316ac5] hover:text-white rounded-sm cursor-pointer group select-none";
      const left = document.createElement("div");
      left.className = "flex items-center gap-1 truncate";
      const dot = document.createElement("span");
      dot.className =
        "w-2 h-2 rounded-full inline-block " +
        (nick === myName
          ? "bg-[#00ff00]"
          : "bg-[#316ac5] group-hover:bg-white");
      const nm = document.createElement("span");
      nm.className =
        "group-hover:text-white font-medium truncate text-[11px]" +
        (nick === myName ? " font-bold" : "");
      nm.style.color = nick === myName ? "#004000" : lineColor(nick);
      nm.textContent = nick + (nick === myName ? " (you)" : "");
      left.appendChild(dot);
      left.appendChild(nm);
      row.appendChild(left);
      rows.appendChild(row);
    }
    $("header-user-count").textContent = members.length;
    $("userlist-count").textContent = members.length;
    $("footer-online-msg").textContent = members.length + " Users Online";
  }
  async function refreshRooms() {
    try {
      const r = await fetch("/rooms");
      const list = await r.json();
      const mine = list.find((x) => x.name === myRoom);
      if (mine) {
        members = mine.members || [];
        renderRoster();
      }
    } catch (e) {}
  }
  window.sendUserMessage = () => {
    const text = input.value;
    if (text === "" || !conn || conn.readyState !== WebSocket.OPEN) return;
    input.value = "";
    conn.send(JSON.stringify({ type: "chat", text }));
  };
  window.sendNudge = async () => {
    try {
      const r = await fetch("/announce", {
        method: "POST",
        body: myName + " sent a NUDGE! :bell:",
      });
      if (r.status !== 202) appendErr("nudge failed: " + r.status);
    } catch (e) {
      appendErr("nudge failed");
    }
    const b = document.getElementById("app-body");
    b.classList.remove("nudge-shake");
    void b.offsetWidth;
    b.classList.add("nudge-shake");
    tone(220, 0.22, "sawtooth", 0.16);
  };
  window.nextRoom = () => {
    const picks = [
      "lobby",
      "music-lounge",
      "nostalgia-2000s",
      "gaming-hub",
    ].filter((r) => r !== myRoom);
    const nxt = picks[Math.floor(Math.random() * picks.length)];
    if (!myName) {
      $("room-input").value = nxt;
      return;
    }
    dial(myName, nxt);
  };
  window.leaveRoomAction = () => {
    if (conn) {
      conn.close(1000, "leaving");
    } else {
      overlay.classList.remove("hidden");
    }
  };
  renderRoster();
  setConn(false);
  setInterval(() => {
    ["led-1", "led-2", "led-3", "led-4", "led-5"].forEach((id, i) => {
      const el = $(id);
      if (el)
        el.style.opacity = i < 1 + Math.floor(Math.random() * 4) ? "1" : "0.2";
    });
  }, 500);
})();
