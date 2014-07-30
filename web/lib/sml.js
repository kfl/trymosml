// CodeMirror, copyright (c) by Marijn Haverbeke and others
// Distributed under an MIT license: http://codemirror.net/LICENSE

(function(mod) {
  if (typeof exports == "object" && typeof module == "object") // CommonJS
    mod(require("../../lib/codemirror"));
  else if (typeof define == "function" && define.amd) // AMD
    define(["../../lib/codemirror"], mod);
  else // Plain browser env
    mod(CodeMirror);
})(function(CodeMirror) {
"use strict";




CodeMirror.defineMode('sml', function(_config, parserConfig) {
  var keywords = {
      'abstype': 'keyword',
      'and': 'keyword',
      'andalso': 'keyword',
      'as': 'keyword',
      'case': 'keyword',
      'datatype': 'keyword',
      'do': 'keyword',
      'else': 'keyword',
      'eqtype': 'keyword',
      'end': 'keyword',
      'exception': 'keyword',
      'fn': 'keyword',
      'fun': 'keyword',
      'functor': 'keyword',
      'handle': 'keyword',
      'if': 'keyword',
      'in': 'keyword',
      'include': 'keyword',
      'infix': 'keyword',
      'infixr': 'keyword',
      'let': 'keyword',
      'local': 'keyword',
      'nonfix': 'keyword',
      'of': 'keyword',
      'op': 'keyword',
      'open': 'keyword',
      'orelse': 'keyword',
      /* Moscow ML specific exclude for now */
      // 'prim_eqtype': 'keyword',
      // 'prim_EQtype': 'keyword',
      // 'prim_type': 'keyword',
      // 'prim_val': 'keyword',
      'raise': 'keyword',
      'rec': 'keyword',
      'sharing': 'keyword',
      'sig': 'keyword',
      'signature': 'keyword',
      'struct': 'keyword',
      'structure': 'keyword',
      'then': 'keyword',
      'type': 'keyword',
      'val': 'keyword',
      'where': 'keyword',
      'while': 'keyword',
      'with': 'keyword',
      'withtype': 'keyword',
      /* We leave out symbolic keywords for now */
      // '#': 'keyword',
      // '->': 'keyword',
      // '|': 'keyword',
      // ':>': 'keyword',
      // ':': 'keyword',
      // '=>': 'keyword',
      // '=': 'keyword'
  };

  function tokenBase(stream, state) {
    var ch = stream.next();

    if (ch === '"') {
      state.tokenize = tokenString;
      return state.tokenize(stream, state);
    }
    if (ch === '(') {
      if (stream.eat('*')) {
        state.commentDepth++;
        state.tokenize = tokenComment;
        return state.tokenize(stream, state);
      }
      return null;
    }
    if (ch === '~') {
        if (stream.eatWhile(/[\d]/)) {
            if (stream.eat('.')) {
                stream.eatWhile(/[\d]/);
            }
            return 'number';
        }
        return null;
    }
    if (/\d/.test(ch)) {
      stream.eatWhile(/[\d]/);
      if (stream.eat('.')) {
        stream.eatWhile(/[\d]/);
      }
      return 'number';
    }
    /* TODO deal with word litterals */

    if (ch === '*') {
      if (stream.eat(')')) {
        return 'error';
      }
      return null;
    }

    // if ( /[+\-*&%=<>!?|]/.test(ch)) {
    //   return 'operator';
    // }

    stream.eatWhile(/\w/);
    var cur = stream.current();
    return keywords[cur] || 'variable';
  }

  function tokenString(stream, state) {
    var next, end = false, escaped = false;
    while ((next = stream.next()) != null) {
      if (next === '"' && !escaped) {
        end = true;
        break;
      }
      escaped = !escaped && next === '\\';
    }
    if (end && !escaped) {
      state.tokenize = tokenBase;
    }



    return 'string';
  };

  function tokenComment(stream, state) {
    var prev, next;
    while(state.commentDepth > 0 && (next = stream.next()) != null) {
      if (prev === '(' && next === '*') state.commentDepth++;
      if (prev === '*' && next === ')') state.commentDepth--;
      prev = next;
    }
    if (state.commentDepth <= 0) {
      state.tokenize = tokenBase;
    }
    return 'comment';
  }

  return {
    startState: function() {return {tokenize: tokenBase, commentDepth: 0};},
    token: function(stream, state) {
      if (stream.eatSpace()) return null;
      return state.tokenize(stream, state);
    },

    blockCommentStart: "(*",
    blockCommentEnd: "*)",
  };
});

CodeMirror.defineMIME('text/x-sml', {
    name: 'sml'
});


});
