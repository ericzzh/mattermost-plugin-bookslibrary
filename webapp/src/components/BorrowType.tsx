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
import { makeStyles, withStyles, styled } from "@material-ui/core/styles";
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
// import Avatar from '@mui/material/Avatar';
import Stepper from "@material-ui/core/Stepper";
import Step from "@material-ui/core/Step";
import StepLabel from "@material-ui/core/StepLabel";
import StepConnector from "@material-ui/core/StepConnector";
import { StepIconProps } from "@material-ui/core/StepIcon";
import clsx from "clsx";
import useMediaQuery from "@material-ui/core/useMediaQuery";
import { useTheme } from "@material-ui/core/styles";
import { red } from "@material-ui/core/colors";
import Accordion from "@material-ui/core/Accordion";
import AccordionSummary from "@material-ui/core/AccordionSummary";
import AccordionDetails from "@material-ui/core/AccordionDetails";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";

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
    renew_request_date: number;
    renew_confirm_date: number;
    return_request_date: number;
    return_confrrm_date: number;
    return_delivery_date: number;
    workflow_type: string;
    workflow: string[];
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

const TEXT: Record<string, string> = {
    BORROW: "借书流程",
    RENEW: "续借流程",
    RETURN: "还书流程",
    REQUESTED: "请求中",
    CONFIRMED: "已确认",
    DELIVIED: "已收到",
    RENEW_REQUESTED: "请求中",
    RENEW_CONFIRMED: "已确认",
    RETURN_REQUESTED: "请求中",
    RETURN_CONFIRMED: "已确认",
    RETURNED: "已还书",
    borrow_requested_date: "借阅请求日",
    borrow_confirmed_date: "借阅确认日",
    borrow_delivied_date: "借阅送达日",
    renew_requested_date: "续借请求日",
    renew_confirmed_date: "续借确认日",
    return_requested_date: "还书请求日",
    return_confirmed_date: "还书确认日",
    return_delivied_date: "还书送达日",
    next_step: "下一步",
};

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
        margin: "auto",
        maxWidth: "100%",
        "& svg": {
            fontSize: "1rem",
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
        maxWidth: "100%",
        maxHeight: "100%",
        float: "left",
    },
    headStatus: {
        backgroundColor: "red",
        [theme.breakpoints.up("xs")]: {
            width: "2rem",
            height: "2rem",
        },
        [theme.breakpoints.up("sm")]: {
            width: "2rem",
            height: "2rem",
        },
    },
    headStatusLabel: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "0.8rem",
        },
    },
    button: {
        marginLeft: 10,
        marginTop: 10,
        width: "10rem",
    },
    bookInfo: {
        [theme.breakpoints.up("xs")]: {
            marginBottom: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            marginBottom: "1.5rem",
        },
    },
    participantsBlock: {
        [theme.breakpoints.up("xs")]: {
            marginBottom: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            marginBottom: "4rem",
        },
    },
    participantChip: {
        [theme.breakpoints.up("xs")]: {
            height: "2rem",
        },
        [theme.breakpoints.up("sm")]: {
            height: "2rem",
        },
    },
    participantLabel: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "0.8rem",
        },
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
            [theme.breakpoints.up("xs")]: {
                fontSize: "1rem",
            },
            [theme.breakpoints.up("sm")]: {
                fontSize: "1.5rem",
            },
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
    const theme = useTheme();
    const matchesSm = useMediaQuery(theme.breakpoints.up("sm"));
    const matchesXs = useMediaQuery(theme.breakpoints.up("xs"));
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
    //

    const StyledWFStepper = styled(Stepper)(({ theme }) => ({
        "& svg": {
            fontSize: "2rem",
        },
        "& .MuiStepLabel-label": {
            fontSize: "1.2rem",
        },
    }));

    const StyledWFButton = styled(Button)(({ theme }) => ({
        alignSelf: "flex-end",
    }));

    const StyledWFTypeChip = styled(Chip)(({theme})=>({
        width: "30%",
        height: "2rem",
        marginTop : "1.5rem",
        fontSize: '1.2rem',
      }))

    const workflow = (
        <Grid container direction={"column"}>
            <StyledWFTypeChip
                variant={"outlined"}
                color="primary"
                label={TEXT[borrow.dataOrImage.workflow_type]}
            />
            <StyledWFStepper
                alternativeLabel
                activeStep={1}
                connector={<ColorlibConnector />}
            >
                {borrow.dataOrImage.workflow?.map((stepLabel) => (
                    <Step key={stepLabel}>
                        <StepLabel StepIconComponent={ColorlibStepIcon}>
                            {TEXT[stepLabel]}
                        </StepLabel>
                    </Step>
                ))}
            </StyledWFStepper>
            <StyledWFButton
                variant={"contained"}
                color={"primary"}
                className={classes.button}
            >
                {TEXT["next_step"]}
            </StyledWFButton>
        </Grid>
    );

    const addDate = (labelid: string, value: number) => {
        if (value === 0) {
            return;
        }
        return (
            <Grid item>
                <TextField
                    label={TEXT[labelid]}
                    value={moment(value).format("YYYY-MM-DD")}
                    InputProps={{
                        readOnly: true,
                    }}
                    className={classes.dateOfRequest}
                />
            </Grid>
        );
    };

    const StyledDateAccordion = styled(Accordion)(({ theme }) => ({
        width: "100%",
        "& .MuiAccordionSummary-root": {
            fontSize: "1.2rem",
        },
    }));

    const dateOfRequest = (
        <Grid container>
            <StyledDateAccordion>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="request_date"
                >
                    <div>{"日期"}</div>
                </AccordionSummary>
                <AccordionDetails>
                    <Grid container spacing={1}>
                        {addDate(
                            "borrow_requested_date",
                            borrow.dataOrImage.request_date
                        )}
                        {addDate(
                            "borrow_confirmed_date",
                            borrow.dataOrImage.confirm_date
                        )}
                        {addDate(
                            "borrow_delivied_date",
                            borrow.dataOrImage.delivery_date
                        )}
                        {addDate(
                            "renew_requested_date",
                            borrow.dataOrImage.renew_request_date
                        )}
                        {addDate(
                            "renew_confirmed_date",
                            borrow.dataOrImage.renew_request_date
                        )}
                        {addDate(
                            "return_requested_date",
                            borrow.dataOrImage.renew_request_date
                        )}
                        {addDate(
                            "return_confirmed_date",
                            borrow.dataOrImage.renew_request_date
                        )}
                        {addDate(
                            "return_delivied_date",
                            borrow.dataOrImage.return_request_date
                        )}
                    </Grid>
                </AccordionDetails>
            </StyledDateAccordion>
        </Grid>
    );

    const headStatus = (
        <Grid container>
            <Grid item>
                <Avatar classes={{ root: classes.headStatus }}>
                    <AlarmOnIcon />
                </Avatar>
            </Grid>
        </Grid>
    );

    const buttonbar = (
        <Grid container justifyContent={"flex-end"}>
            <IconButton>
                <DeleteIcon />
            </IconButton>
        </Grid>
    );

    const titleBar = (
        <Grid item container alignItems={"center"}>
            <Grid item xs={8}>
                {headStatus}
            </Grid>
            <Grid item xs={4}>
                {buttonbar}
            </Grid>
        </Grid>
    );
    const participants = (
        <Grid container spacing={1}>
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
                        root: classes.participantChip,
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
                        root: classes.participantChip,
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
                            root: classes.participantChip,
                        }}
                    />
                ))}
            </Grid>
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
        </Grid>
    );

    const operation = (
        <Grid container direction={"column"} justifyContent={"flex-end"}>
            <Grid item>{dateOfRequest}</Grid>
            <Grid item>{workflow}</Grid>
        </Grid>
    );

    const main = (
        <Grid container spacing={2}>
            <Grid item xs={matchesSm ? 2 : 4} className={classes.img_wrapper}>
                <img
                    className={classes.img}
                    alt="no image"
                    src={`${config.SiteURL}/plugins/${manifest.id}/public/s4216567.jpeg`}
                />
            </Grid>
            <Grid item xs={matchesSm ? 4 : 8}>
                {requestInfo}
            </Grid>
            <Grid item xs={matchesSm ? 6 : 12}>
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
