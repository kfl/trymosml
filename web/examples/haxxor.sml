(* Some useful functions if you want to poke around on the Moscow ML server *)

(* A convenient wrapper for Mosml.run *)
fun cmd str = 
    case Mosml.run str [] "" of
        Mosml.Success res => res
      | Mosml.Failure msg => "Failure: "^msg

val me = ( print(cmd "cat /proc/cpuinfo")
         ; cmd "/usr/bin/whoami");
