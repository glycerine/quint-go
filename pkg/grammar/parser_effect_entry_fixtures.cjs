// Regenerate with: node pkg/grammar/parser_effect_entry_fixtures.cjs
// Actual standalone parseEffect results, including its prefix-acceptance quirk.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const { parseEffect } = require(path.join(root,'quint/src/effects/parser'))
const { EffectLexer } = require(path.join(root,'quint/src/generated/EffectLexer'))
const { EffectParser } = require(path.join(root,'quint/src/generated/EffectParser'))
const { CharStreams, CommonTokenStream, Token } = require(path.join(root,'quint/node_modules/antlr4ts'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const examples = [
  ['pure','Pure'],
  ['effect-variable','_effect42'],
  ['empty-read','Read[]'],
  ['concrete-update',"Update['x']"],
  ['temporal-variable','Temporal[v]'],
  ['mixed-entity-order',"Read['x', v, 'x', w]"],
  ['repeated-entity-and-state',"Read[v, v, 'x', 'x']"],
  ['repeated-and-ordered-components',"Temporal['t'] & Read[r] & Update['u'] & Read['x']"],
  ['zero-arity-arrow','() => Pure'],
  ['arrow-state-occurrence-order',"(Read['x'], Update['x', 'x']) => Temporal['x']"],
  ['nested-arrows',"((Read['a']) => Update['b'], e) => (Pure) => Temporal['c']"],
  ['multiline-whitespace',"( Read[ r, 'x' ],\n\tPure )\r\n=> Update[ 'y', u ]"],
  ['empty-input',''],
  ['missing-read-argument','Read'],
  ['missing-entity-close','Read[v'],
  ['wrong-delimiter','Read(v)'],
  ['empty-state-name',"Update['']"],
  ['missing-state-quote',"Read['x]"],
  ['numeric-state-name',"Read['1']"],
  ['invalid-single-underscore','_'],
  ['leading-entity-comma','Read[,v]'],
  ['leading-update-comma','Update[,v]'],
  ['leading-temporal-comma','Temporal[,v]'],
  ['spaced-leading-comma','Read[ , v]'],
  ['leading-comma-before-component','Read[,v] & Update[u]'],
  ['leading-double-comma','Temporal[,,]'],
  ['leading-comma-after-lexical-error','Read[@,v]'],
  ['leading-comma-arrow-argument','(Read[, Pure) => Update[u]'],
  ['leading-comma-following-argument','(Read[,v], Update[u]) => Pure'],
  ['leading-comma-arrow-result','(Pure) => Read[,v]'],
  ['empty-read-before-parenthesis','Read[)'],
  ['update-before-parenthesis','Update[v)'],
  ['temporal-before-arrow-parenthesis','(Temporal[v) => Pure'],
  ['deleted-read-comma','Read[,]'],
  ['deleted-update-comma','Update[,]'],
  ['deleted-temporal-comma','Temporal[,]'],
  ['deleted-interior-comma','Read[v,,w]'],
  ['missing-bracket-before-component','Read[& Update[u]'],
  ['trailing-entity-comma','Read[v,]'],
  ['trailing-arrow-comma','(Pure,) => Pure'],
  ['missing-arrow-token','(Pure) Pure'],
  ['missing-arrow-result','(Pure) =>'],
  ['missing-arrow-close','(Pure => Pure'],
  ['dangling-component','Read[v] &'],
  ['doubled-component-separator','Read[v] && Update[u]'],
  ['variable-as-component','Read[v] & E'],
  ['lexical-error','Read[@]'],
  ['unicode-lexical-error',"Read['😀']"],
  ['multiline-lexical-error',"Read[v,\n 😀]"],
  ['trailing-effect-variable','Pure extra'],
  ['trailing-effect-component','Pure Read[v]'],
  ['trailing-delimiter','Read[]]'],
  ['trailing-arrow','E => Pure'],
  ['trailing-stateful-arrow',"(Read['x']) => Update['y'] extra"],
  ['trailing-unlexed-junk','Pure extra @'],
  ['trailing-lexical-error','Pure @'],
  ['unicode-after-variable','E😀'],
  ['pure-component-boundary','Pure & Read[v]'],
]

// The public Go diagnostic position is UTF-16 and points at the actual cursor.
// Upstream raw lexer errors instead record index=0; retain those raw locations
// above this transparent source-position projection.
function publicPosition(source,start) {
  const runes=Array.from(source)
  let line=0,col=0,index=0
  for(const rune of runes) {
    if(line===start.line && col===start.col) break
    index+=rune.length
    if(rune==='\n'){line++;col=0}else col++
  }
  const prefix=source.slice(0,index),lines=prefix.split('\n')
  return {line:lines.length-1,col:lines[lines.length-1].length,index}
}

const fixtures=examples.map(([name,source])=>{
  const result=parseEffect(source)
  const rawErrors=result.isLeft()?result.value:[]
  // Observe the generated entry's remaining lookahead without changing the
  // actual parseEffect oracle or walking an alternative effect listener.
  const observed=[]
  const listener={syntaxError:(_,symbol,line,col,explanation)=>{
    const index=symbol?symbol.startIndex:0,len=symbol?symbol.stopIndex-symbol.startIndex:0
    observed.push({explanation,locs:[{start:{line:line-1,col,index},end:{line:line-1,col:col+len,index:index+len}}]})
  }}
  const lexer=new EffectLexer(CharStreams.fromString(source));lexer.removeErrorListeners();lexer.addErrorListener(listener)
  const tokens=new CommonTokenStream(lexer),parser=new EffectParser(tokens)
  parser.removeErrorListeners();parser.addErrorListener(listener);parser.effect()
  const remaining=tokens.LT(1)
  if(JSONbig.stringify(observed)!==JSONbig.stringify(rawErrors))throw Error(name+': entry instrumentation differs')
  const completeInput=remaining.type!==Token.EOF
  const issues=rawErrors.map(error=>({message:error.explanation,position:publicPosition(source,error.locs[0].start)}))
  if(completeInput)issues.push({message:'unexpected trailing input',position:publicPosition(source,{line:remaining.line-1,col:remaining.charPositionInLine})})
  return {name,source,policy:completeInput?'complete-input-boundary':'shared',
    upstream:result.isLeft()?{errors:rawErrors}:{effect:result.value},
    ...(completeInput?{remaining:{text:remaining.text,line:remaining.line-1,col:remaining.charPositionInLine,index:remaining.startIndex}}:{}),issues}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_effect_entry.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} standalone effect entry fixtures`)
