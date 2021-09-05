import React, { useEffect, useState } from "react";
import PropTypes from "prop-types";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import Typography from "@material-ui/core/Typography";
import Breadcrumbs from "@material-ui/core/Breadcrumbs";
import TextField from "@material-ui/core/TextField";
import Link from "@material-ui/core/Link";
import Fab from "@material-ui/core/Fab";
import Chip from "@material-ui/core/Chip";
import FlagIcon from "@material-ui/icons/Flag";
import AlarmOnIcon from "@material-ui/icons/AlarmOn";
import Button from "@material-ui/core/Button";
import IconButton from "@material-ui/core/IconButton";
import DeleteIcon from "@material-ui/icons/Delete";
import Divider from "@material-ui/core/Divider";
import { makeStyles, withStyles } from "@material-ui/core/styles";
import { getCurrentChannel } from "mattermost-redux/selectors/entities/channels";
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
import { searchForTerm } from "actions/post_action";
import { useSelector } from "react-redux";
import WorkerIcon from "@material-ui/icons/PermContactCalendar";
import BorrowIcon from "@material-ui/icons/MenuBook";
import HouseIcon from "@material-ui/icons/House";
import { useDispatch } from "react-redux";
import { getUserByUsername } from "mattermost-redux/selectors/entities/users";
import { GlobalState } from "mattermost-redux/types/store";
import moment from "moment";
import manifest from "../manifest";
import { getConfig } from "mattermost-redux/selectors/entities/general";
import Avatar from "@material-ui/core/Avatar";
import Stepper from "@material-ui/core/Stepper";
import Step from "@material-ui/core/Step";
import StepLabel from "@material-ui/core/StepLabel";
import StepConnector from "@material-ui/core/StepConnector";
import { StepIconProps } from "@material-ui/core/StepIcon";
import clsx from "clsx";

const { formatText, messageHtmlToComponent } = window.PostUtils;

interface BorrowRequest {
    book_post_id: string;
    book_id: string;
    book_name: string;
    author: string;
    borrower_user: string;
    borrower_name: string;
    libworker_user: string;
    libworker_name: string;
    keeper_users: string[];
    keeper_names: string[];
    request_date: number;
    confirm_date: number;
    delivery_date: number;
    renew_date: number;
    return_date: number;
    status: string;
    tags: string[];
}

interface Borrow {
    // put the dataOrImage to be first so as to hide the record of Thread view
    dataOrImage: BorrowRequest;
    role: "MASTER" | "BORROWER" | "LIBWORKER" | "KEEPER";
    relations_keys: {
        book: string;
        master: string;
        borrower: string;
        libworker: string;
        keepers: string;
    };
}

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
        margin: "auto",
        [theme.breakpoints.up("xs")]: { maxWidth: "100%" },
        [theme.breakpoints.up("sm")]: { maxWidth: "100%" },
        "& svg": {
            fontSize: "2rem",
        },
    },
    img_wrapper: {
        [theme.breakpoints.up("xs")]: {
            width: 125 * 0.8,
            height: 160 * 0.8,
        },
        [theme.breakpoints.up("sm")]: {
            width: 125 * 1.5,
            height: 160 * 1.5,
        },
    },
    img: {
        margin: "auto",
        display: "block",
        maxWidth: "100%",
        maxHeight: "100%",
        float: "left",
    },
    button: {
        marginLeft: 10,
        marginTop: 10,
        width: "10rem",
    },
    bookInfo: {
        marginBottom: "4rem",
    },
    participantsBlock: {
        marginBottom: "4rem",
    },
    participantLabel: {
        fontSize: "1.2rem",
    },
    borrowerColor: {
        backgroundColor: "purple",
    },
    libworkerColor: {
        backgroundColor: "green",
    },
    keeperColor: {
        backgroundColor: "teal",
    },
    bookName: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1.5rem",
            fontWeight: "bold",
            marginBottom: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "3rem",
            fontWeight: "bold",
            marginBottom: "2rem",
        },
    },
    authorName: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1rem",
            fontWeight: "bold",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.5rem",
            fontWeight: "bold",
        },
    },
    dateOfRequest: {
        "& label,input": {
            fontSize: "1.3rem",
        },
        [theme.breakpoints.up("xs")]: {
            fontSize: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.5rem",
        },
    },
    stepLabel: {
        "& .MuiStepLabel-label": {
            fontSize: "1.5rem",
        },
    },
}));

const ColorlibConnector = withStyles({
    alternativeLabel: {
        top: 22,
    },
    active: {
        "& $line": {
            backgroundImage:
                "linear-gradient( 95deg,rgb(242,113,33) 0%,rgb(233,64,87) 50%,rgb(138,35,135) 100%)",
        },
    },
    completed: {
        "& $line": {
            backgroundImage:
                "linear-gradient( 95deg,rgb(242,113,33) 0%,rgb(233,64,87) 50%,rgb(138,35,135) 100%)",
        },
    },
    line: {
        height: 3,
        border: 0,
        backgroundColor: "#eaeaf0",
        borderRadius: 1,
    },
})(StepConnector);

const useColorlibStepIconStyles = makeStyles({
    root: {
        backgroundColor: "#ccc",
        zIndex: 1,
        color: "#fff",
        width: 50,
        height: 50,
        display: "flex",
        borderRadius: "50%",
        justifyContent: "center",
        alignItems: "center",
    },
    active: {
        backgroundImage:
            "linear-gradient( 136deg, rgb(242,113,33) 0%, rgb(233,64,87) 50%, rgb(138,35,135) 100%)",
        boxShadow: "0 4px 10px 0 rgba(0,0,0,.25)",
    },
    completed: {
        backgroundImage:
            "linear-gradient( 136deg, rgb(242,113,33) 0%, rgb(233,64,87) 50%, rgb(138,35,135) 100%)",
    },
});

function ColorlibStepIcon(props: StepIconProps) {
    const classes = useColorlibStepIconStyles();
    const { active, completed } = props;

    const icons: { [index: string]: React.ReactElement } = {
        1: <BorrowIcon />,
        2: <WorkerIcon />,
        3: <HouseIcon />,
    };

    return (
        <div
            className={clsx(classes.root, {
                [classes.active]: active,
                [classes.completed]: completed,
            })}
        >
            {icons[String(props.icon)]}
        </div>
    );
}
function BorrowType(props: any) {
    const classes = useStyles();
    const post = { ...props.post };
    const message = post.message || "";
    const config = useSelector(getConfig);
    const currentTeam = useSelector(getCurrentTeam);
    // const currentChannel = useSelector(getCurrentChannel);
    // const formattedText = messageHtmlToComponent(formatText(message));
    //
    // const dispatch = useDispatch();

    let borrow: Borrow;

    try {
        borrow = JSON.parse(message);
    } catch (error) {
        const formattedText = messageHtmlToComponent(formatText(message));
        return <div> {formattedText} </div>;
    }

                // {borrow.dataOrImage.delivery_date !== 0 ? (
                //     <Chip
                //         size={"small"}
                //         icon={<AlarmOnIcon />}
                //         color="primary"
                //         label={moment(borrow.dataOrImage.delivery_date).toNow()}
                //     />
                // ) : (
                //     ""
                // )}
    const borrowStatus = (
        <>
            <Grid item>
                <Chip
                    size={"small"}
                    icon={<FlagIcon />}
                    color="primary"
                    label={borrow.dataOrImage.status}
                />{" "}
                {
                    <Chip
                        size={"small"}
                        icon={<AlarmOnIcon />}
                        color="primary"
                        label={moment(borrow.dataOrImage.delivery_date).toNow()}
                    />
                }
            </Grid>
        </>
    );

    const getSteps = (workflow: string) => {
        switch (workflow) {
            case "Borrow":
                return ["Requested", "Confirmed", "Deliveried"];
            default:
                return ["Requested", "Confirmed", "Deliveried"];
        }
    };

    const buttons = (
        <Grid container direction={"column"}>
            <Stepper
                alternativeLabel
                activeStep={1}
                connector={<ColorlibConnector />}
            >
                {getSteps("Borrow").map((stepLabel) => (
                    <Step key={stepLabel}>
                        <StepLabel
                            StepIconComponent={ColorlibStepIcon}
                            className={classes.stepLabel}
                        >
                            {stepLabel}
                        </StepLabel>
                    </Step>
                ))}
            </Stepper>
            <Button
                variant={"contained"}
                color={"primary"}
                className={classes.button}
            >
                {"Next"}
            </Button>
            <Button
                variant={"outlined"}
                color={"primary"}
                className={classes.button}
            >
                {"Previous"}
            </Button>
        </Grid>
    );

    const addDate = (label: string, value: number) => {
        return (
            <Grid item>
                <TextField
                    label={label}
                    value={moment(value).format("YYYY-MM-DD")}
                    InputProps={{
                        readOnly: true,
                    }}
                    className={classes.dateOfRequest}
                />
            </Grid>
        );
    };

    const dateOfRequest = (
        <Grid container spacing={2}>
            {addDate("申请日", borrow.dataOrImage.request_date)}
            {addDate("确认日", borrow.dataOrImage.confirm_date)}
            {addDate("送达日", borrow.dataOrImage.delivery_date)}
            {addDate("续借日", borrow.dataOrImage.renew_date)}
            {addDate("归还日", borrow.dataOrImage.return_date)}
        </Grid>
    );

    const titleBar = (
        <Grid item container justifyContent={"flex-end"}>
            <Grid item>
                <IconButton>
                    <DeleteIcon />
                </IconButton>
            </Grid>
        </Grid>
    );
    console.log(borrow);
    const participants = (
        <Grid container spacing={2}>
            <Grid item>
                <Chip
                    size={"medium"}
                    // variant={"outlined"}
                    icon={<BorrowIcon />}
                    color="primary"
                    label={borrow.dataOrImage.borrower_name}
                    classes={{
                        label: classes.participantLabel,
                        colorPrimary: classes.borrowerColor,
                    }}
                />
            </Grid>
            <Grid item>
                <Chip
                    size={"medium"}
                    // variant={"outlined"}
                    icon={<WorkerIcon />}
                    color="primary"
                    label={borrow.dataOrImage.libworker_name}
                    classes={{
                        label: classes.participantLabel,
                        colorPrimary: classes.libworkerColor,
                    }}
                />
            </Grid>
            <Grid item>
                {borrow.dataOrImage.keeper_names.map((keeper_name) => (
                    <Chip
                        size={"medium"}
                        // variant={"outlined"}
                        icon={<HouseIcon />}
                        color="primary"
                        label={keeper_name}
                        classes={{
                            label: classes.participantLabel,
                            colorPrimary: classes.keeperColor,
                        }}
                    />
                ))}
            </Grid>
        </Grid>
    );

    const requestStatus = (
        <Grid item container>
            {borrowStatus}
        </Grid>
    );

    // <Link href={`${config.SiteURL}/${currentTeam.id}/pl/${borrow.dataOrImage.book_post_id}`}>
    //     {"转到图书"}
    // </Link>
    const requestInfo = (
        <Grid container direction="column">
            <Grid item className={classes.bookInfo}>
                <div className={classes.bookName}>
                    {borrow.dataOrImage.book_name}
                </div>
                <div className={classes.authorName}>
                    {borrow.dataOrImage.author}
                </div>
            </Grid>
            <Grid item className={classes.participantsBlock}>
                {participants}
            </Grid>
            <Grid item>{requestStatus}</Grid>
        </Grid>
    );

    const operation = (
        <Grid container direction={"column"} justifyContent={"flex-end"}>
            <Grid item>{dateOfRequest}</Grid>
            <Grid item>{buttons}</Grid>
        </Grid>
    );

    const main = (
        <Grid container spacing={2}>
            <Grid item xs={2} className={classes.img_wrapper}>
                <img
                    className={classes.img}
                    alt="no image"
                    src={`${config.SiteURL}/plugins/${manifest.id}/public/s4216567.jpeg`}
                />
            </Grid>
            <Grid item xs={4}>
                {requestInfo}
            </Grid>
            <Grid item xs={6}>
                {operation}
            </Grid>
        </Grid>
    );

    return (
        <Paper className={classes.paper}>
            <Grid container direction={"column"}>
                <Grid item>{titleBar}</Grid>
                <Grid item>{main}</Grid>
            </Grid>
        </Paper>
    );
}

BorrowType.propTypes = {
    post: PropTypes.object.isRequired,
    theme: PropTypes.object.isRequired,
};

export default React.memo(BorrowType);
