module Main exposing (..)

import Html exposing (program)
import Model exposing (Model, Msg(..), init, update)
import View exposing (view)


main : Program Never Model Msg
main =
    program
        { init = init
        , subscriptions = always Sub.none
        , view = view
        , update = update
        }
