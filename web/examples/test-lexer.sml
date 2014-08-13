(* The purpose of this file is to test the syntax highlighting *)

(* Multiline 
   (* (*very*) nested!! (* !*) *)
   comment
*)

(* various litterals *)
val anInt = 4611686018427387903
val negInt = ~4611686018427387904

val pi = 3.14159265358979323846
val eps = 1E~14

val id = fn x => x

val minInt1 = ~1073741824;
val maxInt1 =  1073741823;

val minInt2  = ~0x40000000;
val maxInt21 =  0x3fffffff;
val maxInt22 =  0x3FFFFFFF;

val maxWord1 = 0w2147483647;
val maxWord2 = 0wx7fffffff;

val maxWord8_1 = 0w255;
val maxWord8_2 = 0wxFF;


val hello = "Hello"

val multiline = "Hello \
                \world"

;

(* Some errors that should be highligted by the editor *)

val multOpGotcha = map (op*) [1,2,3];


(* In principle we could highlight these as well, on 32-bit *)

val fail1 = ~1073741825;
val fail2 =  1073741824;
val fail3 = ~0x40000001;
val fail4 =  0x40000000;
val fail5 = 0w2147483648;
val fail6 = 0wx80000000;
val fail7 = 0w256  : Word8.word;
val fail8 = 0wx100 : Word8.word;
val fail9  =  9999999999999999999999999999999999999999;
val fail10 = ~9999999999999999999999999999999999999999;
val fail11 =  0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF;
val fail12 = ~0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF;
val fail13 = 0wxFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF;
val fail14 = 0wxFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF : Word8.word;


