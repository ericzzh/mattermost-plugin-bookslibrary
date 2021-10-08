import React from "react";
import { styled } from "@material-ui/core/styles";
import Snackbar from "@material-ui/core/Snackbar";
import MuiAlert, { AlertProps, Color } from "@material-ui/lab/Alert";

export type MsgBoxProps = {
    open: boolean;
    text: string;
    serverity: Color;
    close?: () => void;
};

function Alert(props: AlertProps) {
    return <MuiAlert elevation={6} variant="filled" {...props} />;
}

export default function MsgBox(props: MsgBoxProps) {
    // const [open, setOpen] = React.useState(props.open);

    const handleMsgClose = (event?: React.SyntheticEvent, reason?: string) => {
        if (reason === "clickaway") {
            return;
        }

        if (props.close) {
            props.close();
        }
        // setOpen(false);
    };

    const StyledSnackbar = styled(Snackbar)(({ theme }) => ({
        position: "absolute",
        "& .MuiAlert-message": {
            fontSize: "1rem",
        },
    }));
    return (
        <StyledSnackbar
            open={props.open}
            autoHideDuration={6000}
            onClose={handleMsgClose}
            anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
        >
            <Alert onClose={handleMsgClose} severity={props.serverity}>
                {props.text}
            </Alert>
        </StyledSnackbar>
    );
}
