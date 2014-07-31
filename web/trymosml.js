var trymosml = function(){
    // Exported functions
    extern = {};
    extern.makeController = makeController;
    extern.connectWebsocket = connectWebsocket;
    extern.makeEditor = makeEditor;
    extern.sendEditorContent = sendEditorContent;
    extern.resetConsole = resetConsole;




    var controller = null;

    // Make the console controller.
    function makeController(){
        controller = $('.mosml-console').console({
            promptLabel: '\u200b',   // zero width space, hack.
            commandHandle: function(line){
                wsSendCommand(line);
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

    function consoleReport(kind, msg) {
        controller.report([{msg: msg, className: kind}]);
    };

    function consoleError(msg) {
	    consoleReport('trymosml-error', msg);
    };

    function consoleInfo(msg) {
	    consoleReport('trymosml-info', msg);
    };

    function consoleWrite(msg) {
	    consoleReport('mosml-output', msg);
    };

    // The mosml websocket
    var ws = null;

    // Send the given line to the server via the ws websocket.
    function wsSendCommand(line) {
	    if (ws) {
		    try {
			    ws.send(line);
		    } catch (ex) {
			    consoleError('Cannot send: ' + ex);
		    }
	    } else {
		    consoleError('Cannot send: not connected to server');
	    }
    };

    function webSocketUrl() {
        var re = new RegExp('^(http)(s)?://([^/]*)(/.*)$');
        var match = location.href.match(re);
        if (match) {
            var wsUrl = 'ws' + (match[2] || '') + '://' + match[3] + match[4];
            return wsUrl;
        } else {
            return 'ws://try.mosml.org';
        }
    }

    function connectWebsocket () {
        var url = webSocketUrl();
	    try {
		    ws = new WebSocket(url);
	    } catch (ex) {
		    consoleError('Cannot connect: ' + ex);
		    ws = null;
		    return;
	    }

        var everConnected = false;

	    ws.onopen = function(ev) {
            everConnected = true;
            $(".run-code").prop('disabled', false);
		    consoleInfo('Connected to Moscow ML server');
	    };
	    ws.onclose = function(ev) {
		    consoleInfo('Connection closed to Moscow ML server');
            $(".run-code").prop('disabled', true);
		    ws = null;
	    };
	    ws.onmessage = function(ev) {
		    consoleWrite(ev.data);
	    };
	    ws.onerror = function(ev) {
		    consoleError('Connection error');
            if (! everConnected) {
                consoleError('Never managed to connect to Moscow ML server');
            }
	    };
    };

    function sendEditorContent() {
        var content = editor.getValue();
        wsSendCommand(content);
    };

    function resetConsole () {
        if (ws) {
            var oldClose = ws.onclose;
            ws.onclose = function () {
                if(oldClose) oldClose();
                controller.reset();
                connectWebsocket();
            };
            ws.close();
        } else {
            controller.reset();
            connectWebsocket();
        }
    };

    var editor = null;

    function makeEditor () {
        var scratchpad = $('.scratchpad')[0];
        editor = CodeMirror.fromTextArea(scratchpad, {
            mode: "text/x-sml",
            autofocus: true,
            lineNumbers: true,
            extraKeys: {
                "Ctrl-B": sendEditorContent
            }
        });
    };

    return extern;
}();


// Main entry point.
$(function(){
    $(".run-code").prop('disabled', true);
    $(".run-code").click(trymosml.sendEditorContent);
    $(".reset-console").click(trymosml.resetConsole);

    trymosml.makeController();
    trymosml.connectWebsocket();
    trymosml.makeEditor();
    
});
