trymosml = {};

// Make the console controller.
trymosml.makeController = function(){
    trymosml.controller = $('.mosml-console').console({
        promptLabel: '\u200b',   // zero width space, hack.
        commandHandle: function(line){
            trymosml.wsSendCommand(line);
            return '\n';
        },
        autofocus: false,
        animateScroll: true,
        promptHistory: true,
        fadeOnReset: false
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
        $(".run-code").prop('disabled', false);
		trymosml.consoleInfo('Connected to Moscow ML server');
	};
	trymosml.ws.onclose = function(ev) {
		trymosml.consoleInfo('Connection closed to Moscow ML server');
        $(".run-code").prop('disabled', true);
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

trymosml.sendEditorContent = function () {
    var content = trymosml.editor.getValue();
    trymosml.wsSendCommand(content);
};

trymosml.resetConsole = function () {
    if (trymosml.ws) {
        var oldClose = trymosml.ws.onclose;
        trymosml.ws.onclose = function () {
            if(oldClose) oldClose();
            trymosml.controller.reset();
            trymosml.connectWebsocket('ws://'+trymosml.url);
        };
        trymosml.ws.close();
    } else {
        trymosml.controller.reset();
        trymosml.connectWebsocket('ws://'+trymosml.url);
    }
};


trymosml.makeEditor = function () {
    var scratchpad = $('.scratchpad')[0];
    trymosml.editor = CodeMirror.fromTextArea(scratchpad, {
        mode: "text/x-sml",
        autofocus: true,
        lineNumbers: true,
        extraKeys: {
            "Ctrl-B": trymosml.sendEditorContent
        }
    });
};


trymosml.url = 'localhost:4242';


// Main entry point.
$(function(){
    $(".run-code").prop('disabled', true);
    $(".run-code").click(trymosml.sendEditorContent);
    $(".reset-console").click(trymosml.resetConsole);

    trymosml.makeController();
    trymosml.connectWebsocket('ws://'+trymosml.url);
    trymosml.makeEditor();
    
});
