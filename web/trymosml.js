trymosml = {};

// Make the console controller.
trymosml.makeController = function(){
    trymosml.controller = $('.mosml-console').console({
        promptLabel: '\u200b',   // zero width space, hack.
        commandHandle: function(line){
            trymosml.wsSendCommand(line);
            return '\n';
        },
        autofocus: true,
        animateScroll: true,
        promptHistory: true
//        welcomeMessage: 'Type SML in here.',
//        continuedPromptLabel: ''
    });
};

trymosml.consoleReport = function(kind, msg) {
    trymosml.controller.report([{msg: msg, className: kind}]);
};

trymosml.consoleError = function(msg) {
	trymosml.consoleReport('trymosml-error', msg);
};

trymosml.consoleInfo = function(msg) {
	trymosml.consoleReport('trymosml-info', msg);
};


trymosml.consoleWrite = function(msg) {
	trymosml.consoleReport('mosml-output', msg);
};

// The mosml websocket
trymosml.ws = null;

// Send the given line to the server via the trymosml.ws websocket.
trymosml.wsSendCommand = function(line) {
	if (trymosml.ws) {
		try {
			trymosml.ws.send(line);
		} catch (ex) {
			trymosml.consoleError('Cannot send: ' + ex);
		}
	} else {
		trymosml.consoleError('Cannot send: not connected to server');
	}
};

trymosml.connectWebsocket = function (url) {
	try {
		trymosml.ws = new WebSocket(url);
	} catch (ex) {
		trymosml.consoleError('Cannot connect: ' + ex);
		trymosml.ws = null;
		return;
	}

    var everConnected = false;

	trymosml.ws.onopen = function(ev) {
        everConnected = true;
		trymosml.consoleInfo('Connected to Moscow ML server');
	};
	trymosml.ws.onclose = function(ev) {
		trymosml.consoleInfo('Connection closed to Moscow ML server');
		trymosml.ws = null;
	};
	trymosml.ws.onmessage = function(ev) {
		trymosml.consoleWrite(ev.data);
	};
	trymosml.ws.onerror = function(ev) {
		trymosml.consoleError('Connection error');
        if (! everConnected) {
            trymosml.consoleError('Never managed to connect to Moscow ML server');
        }
	};
};

// Main entry point.
$(function(){
    var trymosmlUrl = 'localhost:4243';
    trymosml.makeController();
    trymosml.connectWebsocket('ws://'+trymosmlUrl);
});
