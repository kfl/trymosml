(* Some useful functions if you want to poke around on the Moscow ML server *)

fun writeString filename s =
    let val ostr = TextIO.openOut filename
        val _ = TextIO.output(ostr, s)
    in  TextIO.closeOut ostr
    end

val kilroy = writeString "kilroy.txt" "Kilroy was here!"

val I_can_write_files = FileSys.access("kilroy.txt", [FileSys.A_READ])


(* A convenient wrapper for Mosml.run *)
fun cmd str = 
    case Mosml.run str [] "" of
        Mosml.Success res => res
      | Mosml.Failure msg => "Failure: "^msg

val me = ( print(cmd "cat /proc/cpuinfo")
         ; cmd "/usr/bin/whoami");
